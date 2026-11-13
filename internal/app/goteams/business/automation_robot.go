package business

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// dingtalkAt describes the mention options of a DingTalk robot message.
type dingtalkAt struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	AtUserIds []string `json:"atUserIds,omitempty"`
	IsAtAll   bool     `json:"isAtAll"`
}

// dingtalkText describes the text payload of a DingTalk robot message.
type dingtalkText struct {
	Content string `json:"content"`
}

// dingtalkTextMessage describes a DingTalk robot text message.
type dingtalkTextMessage struct {
	MsgType string       `json:"msgtype"`
	Text    dingtalkText `json:"text"`
	At      dingtalkAt   `json:"at"`
}

// feishuText describes the text payload of a Feishu robot message.
type feishuText struct {
	Text string `json:"text"`
}

// feishuTextMessage describes a Feishu robot text message with optional signing fields.
type feishuTextMessage struct {
	MsgType   string     `json:"msg_type"`
	Content   feishuText `json:"content"`
	Timestamp string     `json:"timestamp,omitempty"`
	Sign      string     `json:"sign,omitempty"`
}

// wecomText describes the text payload and mention lists of a WeCom robot message.
type wecomText struct {
	Content             string   `json:"content"`
	MentionedList       []string `json:"mentioned_list,omitempty"`
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
}

// wecomTextMessage describes a WeCom robot text message.
type wecomTextMessage struct {
	MsgType string    `json:"msgtype"`
	Text    wecomText `json:"text"`
}

// automationAtTarget stores the mention parameters a member exposes to each robot API.
type automationAtTarget struct {
	UserID                   int
	Name                     string
	Role                     string // trigger role that introduced this member, empty for explicitly selected members
	Mobile                   string // member profile phone number
	DingtalkAtMobiles        string
	DingtalkAtUserIds        string
	FeishuUserId             string
	WecomMentionedList       string
	WecomMentionedMobileList string
}

// automationAtResolution carries resolved mention targets and every reason a member could not be resolved.
type automationAtResolution struct {
	Targets      []automationAtTarget
	EmptyRoles   []string // trigger roles that carry no user
	MissingUsers []int    // configured members that no longer exist
}

// value returns the identifier the selected at mode carries; the mode already names its robot parameter.
func (target automationAtTarget) value(mode string) string {
	switch mode {
	case "at_mobiles":
		return target.DingtalkAtMobiles
	case "at_user_ids":
		return target.DingtalkAtUserIds
	case "user_id":
		return target.FeishuUserId
	case "mentioned_list":
		return target.WecomMentionedList
	case "mentioned_mobile_list":
		return target.WecomMentionedMobileList
	default:
		return target.Mobile
	}
}

// automationRobotRequest builds the signed URL and text message body of a robot push.
func automationRobotRequest(execution *automationExecution, nodeIndex int, action AutomationAction, values map[string]string, renderedURL string) (string, []byte, error) {
	text := strings.TrimSpace(renderAutomationTemplate(action.Template, values))
	if text == "" {
		return renderedURL, nil, nil
	}
	resolution, err := automationAtTargets(execution, action)
	if err != nil {
		return "", nil, err
	}
	mentions := make([]string, 0, len(resolution.Targets))
	var missing []automationAtTarget
	for _, target := range resolution.Targets {
		if value := target.value(action.AtMode); value != "" {
			mentions = append(mentions, value)
		} else {
			missing = append(missing, target)
		}
	}
	logAutomationAtSkipped(execution, nodeIndex, action, resolution, missing)
	switch action.PushSystem {
	case "dingtalk":
		return automationDingtalkRequest(action, renderedURL, text, mentions)
	case "feishu":
		return automationFeishuRequest(action, renderedURL, text, mentions)
	case "wecom":
		encoded, encodeErr := json.Marshal(automationWecomMessage(action, text, mentions))
		return renderedURL, encoded, encodeErr
	default:
		return "", nil, fmt.Errorf("%w: unknown push system", ErrAutomationConfigInvalid)
	}
}

// automationAtModeLabel returns the readable name of the configured at mode.
func automationAtModeLabel(locale, mode string) string {
	switch mode {
	case "at_mobiles", "at_user_ids", "user_id", "mentioned_list", "mentioned_mobile_list":
		// 参数名本身就是机器人 API 的字段名，不需要翻译。
		return mode
	default:
		return lib_i18n.Show(locale, "automation_at_mode_mobile")
	}
}

// automationAtRoleLabel returns the readable name of a trigger role.
func automationAtRoleLabel(locale, role string) string {
	if role == "assignee" {
		return lib_i18n.Show(locale, "automation_at_role_assignee")
	}
	return lib_i18n.Show(locale, "automation_at_role_created_by")
}

// automationAtSkippedMember describes one member that could not be mentioned.
type automationAtSkippedMember struct {
	UserId int    `json:"user_id"`
	Name   string `json:"name"`
	Role   string `json:"role,omitempty"`
}

// logAutomationAtSkipped records every member that could not be mentioned and the exact reason.
func logAutomationAtSkipped(execution *automationExecution, nodeIndex int, action AutomationAction, resolution automationAtResolution, missing []automationAtTarget) {
	appendLog := func(key string, parameters map[string]any) {
		execution.Logs = append(execution.Logs, AutomationLog{
			Time: time.Now().Unix(), Key: key, Node: nodeIndex + 1, Parameters: parameters,
		})
	}
	if len(resolution.EmptyRoles) > 0 {
		roles := make([]string, 0, len(resolution.EmptyRoles))
		for _, role := range resolution.EmptyRoles {
			roles = append(roles, automationAtRoleLabel(execution.Locale, role))
		}
		appendLog("automation_log_at_role_empty", map[string]any{"roles": roles})
	}
	if len(resolution.MissingUsers) > 0 {
		appendLog("automation_log_at_member_missing", map[string]any{"user_ids": resolution.MissingUsers})
	}
	if len(missing) > 0 {
		members := make([]automationAtSkippedMember, 0, len(missing))
		for _, target := range missing {
			member := automationAtSkippedMember{UserId: target.UserID, Name: target.Name}
			if target.Role != "" {
				member.Role = automationAtRoleLabel(execution.Locale, target.Role)
			}
			members = append(members, member)
		}
		appendLog("automation_log_at_identifier_empty", map[string]any{
			"push_system": action.PushSystem, "at_mode": automationAtModeLabel(execution.Locale, action.AtMode), "members": members,
		})
	}
}

// automationDingtalkRequest builds a DingTalk text message and appends the signature when configured.
// at_user_ids 走 at.atUserIds；mobile 和 at_mobiles 走 at.atMobiles，且 content 内必须出现 @手机号。
// 自动化模板由用户自己撰写，@ 段落跟随模板最后一行。
func automationDingtalkRequest(action AutomationAction, renderedURL, text string, mentions []string) (string, []byte, error) {
	return dingtalkRobotRequest(action, renderedURL, text, mentions, false)
}

// dingtalkRobotRequest composes the DingTalk envelope. ownParagraph puts the system-owned @ block
// on its own line instead of appending it to the last line of the body (external notification copy
// is generated, so @ must not run into the assignee line).
func dingtalkRobotRequest(action AutomationAction, renderedURL, text string, mentions []string, ownParagraph bool) (string, []byte, error) {
	message := dingtalkTextMessage{MsgType: "text", Text: dingtalkText{Content: text}}
	message.At.IsAtAll = action.AtAll
	if ownParagraph && action.AtAll {
		message.Text.Content += "\n"
	}
	if action.AtMode == "at_user_ids" {
		message.At.AtUserIds = mentions
		if ownParagraph && len(mentions) > 0 {
			message.Text.Content += "\n@" + strings.Join(mentions, " @")
		}
	} else {
		message.At.AtMobiles = mentions
		if len(mentions) > 0 {
			separator := " "
			if ownParagraph {
				separator = "\n"
			}
			message.Text.Content += separator + "@" + strings.Join(mentions, " @")
		}
	}
	encoded, err := json.Marshal(message)
	if err != nil {
		return "", nil, err
	}
	signedURL, err := automationDingtalkSignedURL(renderedURL, action.Secret)
	if err != nil {
		return "", nil, err
	}
	return signedURL, encoded, nil
}

// automationFeishuRequest builds a Feishu text message; mentions use at tags inside the text.
// 飞书 @ 全体的写法是 user_id="all"。自动化模板由用户自己撰写，@ 标签放在正文之前。
func automationFeishuRequest(action AutomationAction, renderedURL, text string, mentions []string) (string, []byte, error) {
	return feishuRobotRequest(action, renderedURL, text, mentions, false)
}

// feishuRobotRequest composes the Feishu envelope. ownParagraph moves the @ tags after the body
// onto their own line.
func feishuRobotRequest(action AutomationAction, renderedURL, text string, mentions []string, ownParagraph bool) (string, []byte, error) {
	block := strings.Builder{}
	if action.AtAll {
		block.WriteString(`<at user_id="all"></at> `)
	}
	for _, mention := range mentions {
		block.WriteString(`<at user_id="` + mention + `"></at> `)
	}
	content := block.String() + text
	if ownParagraph && block.Len() > 0 {
		content = text + "\n" + block.String()
	}
	message := feishuTextMessage{MsgType: "text", Content: feishuText{Text: content}}
	if action.Secret != "" {
		timestamp := time.Now().Unix()
		sign, err := automationFeishuSign(action.Secret, timestamp)
		if err != nil {
			return "", nil, err
		}
		message.Timestamp, message.Sign = strconv.FormatInt(timestamp, 10), sign
	}
	encoded, err := json.Marshal(message)
	return renderedURL, encoded, err
}

// automationWecomMessage builds a WeCom text message with the mention list named by the at mode.
// mentioned_list 收 userid，mobile 和 mentioned_mobile_list 收手机号，@ 全体用哨兵值 @all。
func automationWecomMessage(action AutomationAction, text string, mentions []string) wecomTextMessage {
	message := wecomTextMessage{MsgType: "text", Text: wecomText{Content: text}}
	if action.AtAll {
		mentions = append(mentions, "@all")
	}
	if action.AtMode == "mentioned_list" {
		message.Text.MentionedList = mentions
	} else {
		message.Text.MentionedMobileList = mentions
	}
	return message
}

// automationDingtalkSignedURL appends timestamp and sign following the DingTalk signing rule.
// Sign = urlEncode(base64(HmacSHA256(key: secret, data: timestamp+"\n"+secret))), timestamp in milliseconds.
func automationDingtalkSignedURL(renderedURL, secret string) (string, error) {
	if secret == "" {
		return renderedURL, nil
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(timestamp + "\n" + secret)); err != nil {
		return "", err
	}
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	separator := "?"
	if strings.Contains(renderedURL, "?") {
		separator = "&"
	}
	return renderedURL + separator + "timestamp=" + timestamp + "&sign=" + url.QueryEscape(sign), nil
}

// automationFeishuSign signs an empty payload following the Feishu signing rule.
// Sign = base64(HmacSHA256(key: timestamp+"\n"+secret, data: empty)), timestamp in seconds.
func automationFeishuSign(secret string, timestamp int64) (string, error) {
	mac := hmac.New(sha256.New, []byte(strconv.FormatInt(timestamp, 10)+"\n"+secret))
	if _, err := mac.Write(nil); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// automationAtTargets resolves the mentioned members configured as explicit users or trigger roles.
// Members that cannot be resolved are reported instead of being dropped silently.
func automationAtTargets(execution *automationExecution, action AutomationAction) (automationAtResolution, error) {
	resolution := automationAtResolution{}
	seen := make(map[int]bool, len(action.AtUserIds)+len(action.AtRoles))
	roles := make(map[int]string, len(action.AtRoles))
	userIDs := make([]int, 0, len(action.AtUserIds)+len(action.AtRoles))
	add := func(userID int, role string) {
		if userID <= 0 || seen[userID] {
			return
		}
		seen[userID] = true
		userIDs = append(userIDs, userID)
		if role != "" {
			roles[userID] = role
		}
	}
	for _, userID := range action.AtUserIds {
		add(userID, "")
	}
	if len(action.AtRoles) > 0 {
		trigger := execution.Snapshot.Event.After
		if trigger == nil {
			trigger = execution.Snapshot.Event.Before
		}
		for _, role := range action.AtRoles {
			userID := 0
			if trigger != nil {
				userID = cast.ToInt(trigger.Fields[role])
			}
			if userID <= 0 {
				resolution.EmptyRoles = append(resolution.EmptyRoles, role)
				continue
			}
			add(userID, role)
		}
	}
	if len(userIDs) == 0 {
		return resolution, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id,name,tel_number,
		push_dingtalk_at_mobiles,push_dingtalk_at_user_ids,push_feishu_user_id,
		push_wecom_mentioned_list,push_wecom_mentioned_mobile_list
		FROM users WHERE admin_id=$1 AND id=ANY($2)`, nil, execution.AdminID, pq.Array(userIDs))
	if err != nil {
		logs.Error("query automation at targets failed: %s", err.Error())
		return automationAtResolution{}, err
	}
	indexed := make(map[int]automationAtTarget, len(rows))
	for _, row := range rows {
		id, convErr := strconv.Atoi(row["id"])
		if convErr != nil {
			return automationAtResolution{}, fmt.Errorf("parse at target id: %w", convErr)
		}
		indexed[id] = automationAtTarget{
			UserID: id, Name: row["name"], Mobile: row["tel_number"],
			DingtalkAtMobiles:        row["push_dingtalk_at_mobiles"],
			DingtalkAtUserIds:        row["push_dingtalk_at_user_ids"],
			FeishuUserId:             row["push_feishu_user_id"],
			WecomMentionedList:       row["push_wecom_mentioned_list"],
			WecomMentionedMobileList: row["push_wecom_mentioned_mobile_list"],
		}
	}
	resolution.Targets = make([]automationAtTarget, 0, len(userIDs))
	for _, userID := range userIDs {
		target, ok := indexed[userID]
		if !ok {
			resolution.MissingUsers = append(resolution.MissingUsers, userID)
			continue
		}
		target.Role = roles[userID]
		resolution.Targets = append(resolution.Targets, target)
	}
	return resolution, nil
}
