package business

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net"
	"net/url"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// 个性化错误只暴露稳定的业务原因，数据库和存储错误由 controller 统一处理。
var (
	ErrPersonalizationEnterprise  = errors.New("personalization requires an active enterprise plan")
	ErrPersonalizationDomainTaken = errors.New("personalization domain already in use")
	ErrPersonalizationLogo        = errors.New("personalization logo must be PNG or JPEG, up to 2MB")
)

// PersonalizationLogoMaxSize 是应用图标的上传上限。
const PersonalizationLogoMaxSize int64 = 2 * 1024 * 1024

// PersonalizationModel 是租户的品牌配置，Logo 保存存储路径而非临时签名 URL。
type PersonalizationModel struct {
	AdminId      int
	AppName      string
	Logo         string
	DomainPrefix string
	LoginSlogan  string
	Editable     bool
}

// PersonalizationSiteURL 以部署站点为域名后缀，不信任客户端提供的 Host 拼接回跳地址。
func PersonalizationSiteURL(prefix string) string {
	site, err := url.Parse(strings.TrimSpace(define.Config.WebService["site_url"]))
	if err != nil || (site.Scheme != "http" && site.Scheme != "https") || site.Hostname() == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(site.Hostname()), "www.")
	if net.ParseIP(host) != nil {
		return ""
	}
	if prefix != "" {
		host = prefix + "." + host
	}
	if site.Port() != "" {
		host = net.JoinHostPort(host, site.Port())
	}
	return site.Scheme + "://" + host
}

// PersonalizationDomainPrefix 仅识别部署域名下的单层企业子域名。
func PersonalizationDomainPrefix(host string) string {
	base, err := url.Parse(PersonalizationSiteURL(""))
	if err != nil || base.Hostname() == "" {
		return ""
	}
	if parsed, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		host = parsed
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	suffix := "." + base.Hostname()
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	prefix := strings.TrimSuffix(host, suffix)
	if prefix == "" || validate.ValidatePersonalization("", prefix, "") != nil {
		return ""
	}
	return prefix
}

func loadPersonalization(adminId int) (PersonalizationModel, error) {
	m := msql.Model(define.TableEnterprisePersonalizations, define.Postgres)
	row, err := m.Field("app_name, logo, domain_prefix, login_slogan").Where("admin_id", cast.ToString(adminId)).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return PersonalizationModel{}, err
	}
	return PersonalizationModel{AdminId: adminId, AppName: row["app_name"], Logo: row["logo"], DomainPrefix: row["domain_prefix"], LoginSlogan: row["login_slogan"]}, nil
}

// GetPersonalization 读取管理表单，包括套餐只读状态。
func GetPersonalization(adminId int) (PersonalizationModel, error) {
	plan, err := GetPlan(adminId)
	if err != nil {
		return PersonalizationModel{}, err
	}
	model, err := loadPersonalization(adminId)
	model.Editable = plan.PlanType == define.PlanEnterprise && !plan.IsExpired
	return model, err
}

// GetTenantBranding 开源版统一默认品牌；旧配置保留但不读取应用。
func GetTenantBranding(adminId int) (PersonalizationModel, error) {
	return PersonalizationModel{}, nil
}

// GetPublicBranding 按专属域名只读查询租户，不为匿名访问物化套餐或配置。
func GetPublicBranding(host string) (PersonalizationModel, error) {
	prefix := PersonalizationDomainPrefix(host)
	if prefix == "" {
		return PersonalizationModel{}, nil
	}
	m := msql.Model(define.TableEnterprisePersonalizations+" p", define.Postgres)
	row, err := m.Join(define.TableTenantPlans+" tp", "tp.admin_id=p.admin_id", "inner").
		Field("p.admin_id, p.app_name, p.logo, p.domain_prefix, p.login_slogan, tp.expire_at").
		Where("p.domain_prefix", prefix).Where("tp.plan_type", define.PlanEnterprise).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return PersonalizationModel{}, err
	}
	if len(row) == 0 {
		return PersonalizationModel{}, nil
	}
	expireAt, err := parseRowInt(row, "expire_at")
	if err != nil {
		return PersonalizationModel{}, err
	}
	if isPlanExpired(expireAt) {
		return PersonalizationModel{}, nil
	}
	adminId, err := parseRowInt(row, "admin_id")
	if err != nil {
		return PersonalizationModel{}, err
	}
	return PersonalizationModel{AdminId: adminId, AppName: row["app_name"], Logo: row["logo"], DomainPrefix: row["domain_prefix"], LoginSlogan: row["login_slogan"]}, nil
}

// SavePersonalization 保存整份配置；数据库唯一索引裁决并发域名占用。
func SavePersonalization(model PersonalizationModel) (PersonalizationModel, error) {
	current, err := GetPersonalization(model.AdminId)
	if err != nil {
		return PersonalizationModel{}, err
	}
	if !current.Editable {
		return PersonalizationModel{}, ErrPersonalizationEnterprise
	}
	model.AppName = strings.TrimSpace(model.AppName)
	model.DomainPrefix = strings.ToLower(strings.TrimSpace(model.DomainPrefix))
	if err = validate.ValidatePersonalization(model.AppName, model.DomainPrefix, model.LoginSlogan); err != nil {
		return PersonalizationModel{}, err
	}
	if model.DomainPrefix != "" && PersonalizationSiteURL(model.DomainPrefix) == "" {
		return PersonalizationModel{}, &validate.FieldError{Key: "personalization_domain_unavailable"}
	}
	if model.Logo != "" && (!lib_storage.M.IsOwned(model.AdminId, lib_storage.BusinessPersonalization, model.Logo) || !lib_storage.M.Exists(model.Logo)) {
		return PersonalizationModel{}, ErrPersonalizationLogo
	}
	const query = `INSERT INTO ` + define.TableEnterprisePersonalizations + ` (admin_id,app_name,logo,domain_prefix,login_slogan,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$6) ON CONFLICT(admin_id) DO UPDATE SET
		app_name=EXCLUDED.app_name, logo=EXCLUDED.logo, domain_prefix=EXCLUDED.domain_prefix,
		login_slogan=EXCLUDED.login_slogan, updated_at=EXCLUDED.updated_at`
	_, err = msql.RawExec(define.Postgres, query, nil, model.AdminId, model.AppName, model.Logo, model.DomainPrefix, model.LoginSlogan, time.Now().Unix())
	if err != nil {
		var dbErr *pq.Error
		if errors.As(err, &dbErr) && dbErr.Code == "23505" && dbErr.Constraint == "enterprise_personalizations_domain_unique" {
			return PersonalizationModel{}, ErrPersonalizationDomainTaken
		}
		logs.Error("sql:%s, err:%s", query, err.Error())
		return PersonalizationModel{}, err
	}
	model.Editable = true
	return model, nil
}

// UploadPersonalizationLogo 暂存应用图标；只有保存设置才更新当前品牌。
func UploadPersonalizationLogo(adminId int, file *multipart.FileHeader) (string, error) {
	config, err := GetPersonalization(adminId)
	if err != nil {
		return "", err
	}
	if !config.Editable {
		return "", ErrPersonalizationEnterprise
	}
	if file == nil || file.Size <= 0 || file.Size > PersonalizationLogoMaxSize {
		return "", ErrPersonalizationLogo
	}
	opened, err := file.Open()
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := opened.Close(); closeErr != nil {
			logs.Error("close personalization logo: %s", closeErr.Error())
		}
	}()
	info, format, err := image.DecodeConfig(opened)
	if err != nil || (format != "png" && format != "jpeg") || info.Width > 4096 || info.Height > 4096 {
		return "", ErrPersonalizationLogo
	}
	if _, err = opened.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return lib_storage.M.Upload(adminId, lib_storage.BusinessPersonalization, "."+format, opened)
}
