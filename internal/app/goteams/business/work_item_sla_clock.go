package business

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"goteams/internal/app/goteams/define"
)

type slaHolidaySource struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
}

type slaHolidayYear struct {
	Year                int              `json:"year"`
	Revision            string           `json:"revision"`
	Source              slaHolidaySource `json:"source"`
	ConfirmedNoHolidays bool             `json:"confirmed_no_holidays"`
	Dates               []string         `json:"holiday_dates"`
	SHA256              string           `json:"sha256,omitempty"`
}

type slaHolidayCalendar struct {
	Code        string           `json:"code"`
	Region      string           `json:"region"`
	Description string           `json:"description"`
	Years       []slaHolidayYear `json:"years"`
}

type slaHolidayFile struct {
	SchemaVersion   int                  `json:"schema_version"`
	DefaultCalendar string               `json:"default_calendar"`
	Calendars       []slaHolidayCalendar `json:"calendars"`
}

var slaHolidayCache struct {
	sync.Once
	data slaHolidayFile
	err  error
}

func loadSLAHolidays() (slaHolidayFile, error) {
	slaHolidayCache.Do(func() {
		data, err := os.ReadFile(filepath.Join(define.AppRoot, "configs", "goteams", "sla_holidays.json"))
		if err != nil {
			slaHolidayCache.err = fmt.Errorf("load SLA holidays: %w", err)
			return
		}
		slaHolidayCache.data, slaHolidayCache.err = parseSLAHolidays(data)
	})
	return slaHolidayCache.data, slaHolidayCache.err
}

func parseSLAHolidays(data []byte) (slaHolidayFile, error) {
	var file slaHolidayFile
	if len(data) > 2<<20 {
		return file, ErrSLAInvalid
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return file, fmt.Errorf("parse SLA holidays: %w", err)
	}
	if file.SchemaVersion != 1 || len(file.Calendars) == 0 || len(file.Calendars) > 32 {
		return file, ErrSLAInvalid
	}
	codes := map[string]bool{}
	for i := range file.Calendars {
		calendar := &file.Calendars[i]
		if calendar.Code == "" || len(calendar.Code) > 64 || calendar.Region == "" || codes[calendar.Code] {
			return file, ErrSLAInvalid
		}
		codes[calendar.Code] = true
		years := map[int]bool{}
		for j := range calendar.Years {
			year := &calendar.Years[j]
			if years[year.Year] || year.Year < 1970 || year.Year > 9998 || year.Revision == "" || year.Source.Name == "" || year.Source.Reference == "" {
				return file, ErrSLAInvalid
			}
			years[year.Year] = true
			if (len(year.Dates) == 0) != year.ConfirmedNoHolidays || len(year.Dates) > 366 {
				return file, ErrSLAInvalid
			}
			seen := map[string]bool{}
			dates := make([]string, 0, len(year.Dates))
			for _, date := range year.Dates {
				parsed, err := time.Parse("2006-01-02", date)
				if err != nil || parsed.Year() != year.Year || parsed.Format("2006-01-02") != date {
					return file, ErrSLAInvalid
				}
				if !seen[date] {
					dates = append(dates, date)
					seen[date] = true
				}
			}
			sort.Strings(dates)
			year.Dates = dates
			canonical := fmt.Sprintf("%s\n%s\n%d\n%s\n%t\n%s", calendar.Code, calendar.Region, year.Year, year.Revision, year.ConfirmedNoHolidays, strings.Join(dates, ","))
			hash := sha256.Sum256([]byte(canonical))
			year.SHA256 = hex.EncodeToString(hash[:])
		}
	}
	if !codes[file.DefaultCalendar] {
		return file, ErrSLAInvalid
	}
	return file, nil
}

// bindSLAHolidayYears 只能由持周期锁的写入路径调用，不修改已绑定年份。
func bindSLAHolidayYears(calendar *slaCalendar) (bool, error) {
	if !calendar.PauseOnHolidays {
		return false, nil
	}
	file, err := loadSLAHolidays()
	if err != nil {
		return false, err
	}
	code := calendar.HolidayCalendarCode
	if code == "" || code == "default" {
		code = file.DefaultCalendar
	}
	for _, source := range file.Calendars {
		if source.Code != code {
			continue
		}
		bound := map[int]bool{}
		for _, year := range calendar.HolidayYears {
			bound[year.Year] = true
		}
		changed := false
		for _, year := range source.Years {
			if !bound[year.Year] {
				calendar.HolidayYears = append(calendar.HolidayYears, year)
				changed = true
			}
		}
		calendar.HolidayCalendarCode = code
		return changed, nil
	}
	return false, ErrSLACalendar
}

func slaHoliday(calendar slaCalendar, day time.Time) (bool, error) {
	if !calendar.PauseOnHolidays {
		return false, nil
	}
	for _, year := range calendar.HolidayYears {
		if year.Year == day.Year() {
			date := day.Format("2006-01-02")
			index := sort.SearchStrings(year.Dates, date)
			return index < len(year.Dates) && year.Dates[index] == date, nil
		}
	}
	return false, ErrSLACalendar
}

func mergeSLAIntervals(intervals []slaPause) []slaPause {
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].Start < intervals[j].Start })
	merged := make([]slaPause, 0, len(intervals))
	for _, interval := range intervals {
		if interval.End <= interval.Start {
			continue
		}
		if len(merged) > 0 && interval.Start <= merged[len(merged)-1].End {
			if interval.End > merged[len(merged)-1].End {
				merged[len(merged)-1].End = interval.End
			}
		} else {
			merged = append(merged, interval)
		}
	}
	return merged
}

// walkSLAIntervals 遍历真实日历的有效片段，visitor 返回 false 时提前结束。
func walkSLAIntervals(calendar slaCalendar, pauses []slaPause, start, end int64, visitor func(int64, int64) bool) error {
	if end <= start {
		return nil
	}
	location, err := time.LoadLocation(calendar.Timezone)
	if err != nil {
		return fmt.Errorf("SLA timezone: %w", err)
	}
	if calendar.Mode != "all_day" && calendar.Mode != "weekly" {
		return ErrSLAInvalid
	}
	blocked := make([]slaPause, 0, len(pauses))
	for _, pause := range pauses {
		if pause.End == 0 {
			pause.End = end
		}
		blocked = append(blocked, pause)
	}
	blocked = mergeSLAIntervals(blocked)
	day := time.Unix(start, 0).In(location)
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, location)
	for count := 0; day.Unix() < end; count++ {
		// 防止极端配置造成不受限的计算，正常周期远小于此范围。
		if count >= 36600 {
			return ErrSLAInvalid
		}
		next := day.AddDate(0, 0, 1)
		windows := []slaPause{}
		if calendar.Mode == "all_day" {
			windows = append(windows, slaPause{day.Unix(), next.Unix()})
		} else {
			weekday := int(day.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			for _, slot := range calendar.Slots {
				for _, selected := range slot.Weekdays {
					if selected == weekday {
						from := time.Date(day.Year(), day.Month(), day.Day(), slot.StartMinute/60, slot.StartMinute%60, 0, 0, location)
						to := time.Date(day.Year(), day.Month(), day.Day(), slot.EndMinute/60, slot.EndMinute%60, 0, 0, location)
						windows = append(windows, slaPause{from.Unix(), to.Unix()})
						break
					}
				}
			}
		}
		effective := []slaPause{}
		for _, window := range mergeSLAIntervals(windows) {
			from, to := max(start, window.Start), min(end, window.End)
			if to <= from {
				continue
			}
			for _, pause := range blocked {
				if pause.Start >= to {
					break
				}
				if pause.End <= from {
					continue
				}
				if pause.Start > from {
					effective = append(effective, slaPause{from, pause.Start})
				}
				from = max(from, pause.End)
				if from >= to {
					break
				}
			}
			if from < to {
				effective = append(effective, slaPause{from, to})
			}
		}
		// 没有可能累计的片段时无需年度日历，完整关闭年份和非工作日不造成虚假的待计算。
		if len(effective) > 0 {
			holiday, err := slaHoliday(calendar, day)
			if err != nil {
				return err
			}
			if !holiday {
				for _, interval := range effective {
					if !visitor(interval.Start, interval.End) {
						return nil
					}
				}
			}
		}
		day = next
	}
	return nil
}

func slaElapsed(calendar slaCalendar, pauses []slaPause, start, end int64) (int64, error) {
	var elapsed int64
	err := walkSLAIntervals(calendar, pauses, start, end, func(from, to int64) bool {
		elapsed += to - from
		return true
	})
	return elapsed, err
}

func slaDeadline(calendar slaCalendar, pauses []slaPause, start, seconds int64) (int64, error) {
	if seconds < 0 {
		return 0, ErrSLAInvalid
	}
	if seconds == 0 {
		return start, nil
	}
	var deadline int64
	err := walkSLAIntervals(calendar, pauses, start, start+36600*86400, func(from, to int64) bool {
		if seconds <= to-from {
			deadline = from + seconds
			return false
		}
		seconds -= to - from
		return true
	})
	if err != nil {
		return 0, err
	}
	if deadline == 0 {
		return 0, ErrSLAInvalid
	}
	return deadline, nil
}
