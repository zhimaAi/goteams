package business

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// SatisfactionStatisticsQueryModel 限定一次只读统计的租户、时间及快照筛选。
type SatisfactionStatisticsQueryModel struct {
	AdminId, WorkspaceId, UserId             int
	From, To                                 int64
	Timezone                                 string
	MemberId, RoleId, CategoryId, PriorityId *int
	Page, Size                               int
}

// SatisfactionCountModel 所有数量均按工单去重，不按邀请动作行数累计。
type SatisfactionCountModel struct{ Invited, Submitted, Positive, Neutral, Negative int }

// SatisfactionGroupModel 表示人员、分类或日期的一组计数。
type SatisfactionGroupModel struct {
	Id    int
	Name  string
	Count SatisfactionCountModel
}

// SatisfactionChoiceModel 是历史快照中可用的筛选选项。
type SatisfactionChoiceModel struct {
	Id   int
	Name string
}

// SatisfactionStatisticsModel 是固定统计页的数据，不依赖SLA配置。
type SatisfactionStatisticsModel struct {
	Count                                                        SatisfactionCountModel
	Trend, Members, Categories                                   []SatisfactionGroupModel
	MemberOptions, RoleOptions, CategoryOptions, PriorityOptions []SatisfactionChoiceModel
	MemberTotal, CategoryTotal, Page, Size                       int
	EvaluatedAt                                                  int64
}

func (c *SatisfactionCountModel) add(submitted int64, sentiment string) {
	c.Invited++
	if submitted == 0 {
		return
	}
	c.Submitted++
	switch sentiment {
	case "positive":
		c.Positive++
	case "neutral":
		c.Neutral++
	case "negative":
		c.Negative++
	}
}
func satisfactionChoices(m map[int]string) []SatisfactionChoiceModel {
	out := []SatisfactionChoiceModel{}
	for id, name := range m {
		out = append(out, SatisfactionChoiceModel{Id: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}
func satisfactionGroups(m map[int]*SatisfactionGroupModel, page, size int) ([]SatisfactionGroupModel, int) {
	out := []SatisfactionGroupModel{}
	for _, g := range m {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count.Invited != out[j].Count.Invited {
			return out[i].Count.Invited > out[j].Count.Invited
		}
		return out[i].Id < out[j].Id
	})
	total := len(out)
	start := (page - 1) * size
	if start >= total {
		return []SatisfactionGroupModel{}, total
	}
	end := start + size
	if end > total {
		end = total
	}
	return out[start:end], total
}
func satisfactionAddGroup(groups map[int]*SatisfactionGroupModel, id int, name string, submitted int64, sentiment string) {
	g := groups[id]
	if g == nil {
		g = &SatisfactionGroupModel{Id: id, Name: name}
		groups[id] = g
	}
	g.Count.add(submitted, sentiment)
}

// QuerySatisfactionStatistics 以首次开放时间建立同批分母，角色和成员必须命中同一个人。
func QuerySatisfactionStatistics(ctx context.Context, m SatisfactionStatisticsQueryModel) (SatisfactionStatisticsModel, error) {
	result := SatisfactionStatisticsModel{Trend: []SatisfactionGroupModel{}, Members: []SatisfactionGroupModel{}, Categories: []SatisfactionGroupModel{}, EvaluatedAt: time.Now().Unix(), Page: m.Page, Size: m.Size}
	if m.Page < 1 || m.Page > 100000 || m.Size < 1 || m.Size > 100 || m.From <= 0 || m.To <= m.From || m.To-m.From > 367*86400 {
		return result, ErrSatisfactionInvalid
	}
	loc, e := time.LoadLocation(m.Timezone)
	if e != nil {
		return result, ErrSatisfactionInvalid
	}
	for _, id := range []*int{m.MemberId, m.RoleId, m.CategoryId, m.PriorityId} {
		if id != nil && *id < 0 {
			return result, ErrSatisfactionInvalid
		}
	}
	if !HasPermission(m.AdminId, m.UserId, strconv.Itoa(m.WorkspaceId), "issue_statistics:view") || !HasPermission(m.AdminId, m.UserId, strconv.Itoa(m.WorkspaceId), "requirement:view") {
		return result, ErrSatisfactionForbidden
	}
	if e = ensureSatisfactionWorkspace(nil, m.AdminId, m.WorkspaceId); e != nil {
		return result, e
	}
	where, args, e := buildDataViewScopeCondition(m.AdminId, strconv.Itoa(m.WorkspaceId), m.UserId, 5)
	if e != nil {
		return result, e
	}
	q := `SELECT s.eligible_at,s.submitted_at,COALESCE(s.sentiment,''),s.category_id,s.category_name,s.priority_id,s.priority_name,s.handlers_snapshot FROM ` + define.TableSatisfactions + ` s JOIN ` + define.TableRequirements + ` r ON r.admin_id=s.admin_id AND r.workspace_id=s.workspace_id AND r.id=s.requirement_id
 JOIN ` + define.TableWorkspaceIssueRequirements + ` ir ON ir.admin_id=s.admin_id AND ir.workspace_id=s.workspace_id AND ir.requirement_id=s.requirement_id AND ir.openid=s.openid
 WHERE s.admin_id=$1 AND s.workspace_id=$2 AND s.eligible_at >= $3 AND s.eligible_at < $4`
	if where != "" {
		q += " AND " + where
	}
	params := []any{m.AdminId, m.WorkspaceId, m.From, m.To}
	params = append(params, args...)
	db, e := msql.GetDB(define.Postgres)
	if e != nil {
		return result, e
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	rows, e := db.QueryContext(ctx, q, params...)
	if e != nil {
		logs.Error("sql:%s, err:%s", q, e.Error())
		return result, e
	}
	defer rows.Close()
	members, categories := map[int]*SatisfactionGroupModel{}, map[int]*SatisfactionGroupModel{}
	days := map[string]*SatisfactionGroupModel{}
	memberOpts, roleOpts, categoryOpts, priorityOpts := map[int]string{}, map[int]string{}, map[int]string{}, map[int]string{}
	for day := time.Unix(m.From, 0).In(loc); day.Unix() < m.To; day = day.AddDate(0, 0, 1) {
		name := day.Format(time.DateOnly)
		days[name] = &SatisfactionGroupModel{Name: name}
	}
	for rows.Next() {
		var eligible, submitted int64
		var sentiment, catName, priorityName string
		var cat, priority int
		var raw []byte
		if e = rows.Scan(&eligible, &submitted, &sentiment, &cat, &catName, &priority, &priorityName, &raw); e != nil {
			return result, e
		}
		var handlers []SatisfactionHandlerModel
		if e = json.Unmarshal(raw, &handlers); e != nil {
			return result, e
		}
		if len(handlers) == 0 {
			handlers = []SatisfactionHandlerModel{{}}
		}
		categoryOpts[cat] = catName
		priorityOpts[priority] = priorityName
		matched := m.MemberId == nil && m.RoleId == nil
		for _, h := range handlers {
			memberOpts[h.UserId] = h.UserName
			roleOpts[h.RoleId] = h.RoleName
			if (m.MemberId == nil || *m.MemberId == h.UserId) && (m.RoleId == nil || *m.RoleId == h.RoleId) {
				matched = true
			}
		}
		if !matched || (m.CategoryId != nil && *m.CategoryId != cat) || (m.PriorityId != nil && *m.PriorityId != priority) {
			continue
		}
		result.Count.add(submitted, sentiment)
		name := time.Unix(eligible, 0).In(loc).Format(time.DateOnly)
		if day := days[name]; day != nil {
			day.Count.add(submitted, sentiment)
		}
		satisfactionAddGroup(categories, cat, catName, submitted, sentiment)
		seen := map[int]bool{}
		for _, h := range handlers {
			if (m.MemberId != nil && *m.MemberId != h.UserId) || (m.RoleId != nil && *m.RoleId != h.RoleId) {
				continue
			}
			if seen[h.UserId] {
				continue
			}
			seen[h.UserId] = true
			satisfactionAddGroup(members, h.UserId, h.UserName, submitted, sentiment)
		}
	}
	if e = rows.Err(); e != nil {
		return result, e
	}
	for _, day := range days {
		result.Trend = append(result.Trend, *day)
	}
	sort.Slice(result.Trend, func(i, j int) bool { return result.Trend[i].Name < result.Trend[j].Name })
	result.Members, result.MemberTotal = satisfactionGroups(members, m.Page, m.Size)
	result.Categories, result.CategoryTotal = satisfactionGroups(categories, m.Page, m.Size)
	result.MemberOptions = satisfactionChoices(memberOpts)
	result.RoleOptions = satisfactionChoices(roleOpts)
	result.CategoryOptions = satisfactionChoices(categoryOpts)
	result.PriorityOptions = satisfactionChoices(priorityOpts)
	return result, nil
}
