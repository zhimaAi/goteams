package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegPlanRoutes 注册版本信息与企业名称接口。
func RegPlanRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/enterprise/plan/info`] = wrapAction(withAuth(), controller.GetPlanInfo)
	r[http.MethodGet][`/api/enterprise/plan/usage`] = wrapAction(withPerm("plan:view"), controller.GetPlanUsage)
	r[http.MethodPut][`/api/enterprise/plan/storage-extend`] = wrapAction(withAuth(), controller.ExtendStorageDeadline)
	r[http.MethodPut][`/api/enterprise/name`] = wrapAction(withAuth(), controller.UpdateEnterpriseName)
}
