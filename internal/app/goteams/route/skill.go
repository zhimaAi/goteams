package route

import (
	"net/http"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
)

// RegSkillRoutes 注册系统级技能接口；列表和创建匿名开放，详情、修改和删除使用开放接口验签。
func RegSkillRoutes(r lib_web.Route) {
	signed := common.OpenApiSignMiddleware()
	r[http.MethodGet]["/api/skills"] = controller.ListSkills
	r[http.MethodPost]["/api/skills"] = controller.CreateSkill
	r[http.MethodGet]["/api/skills/:skillId"] = wrapAction(signed, controller.GetSkill)
	r[http.MethodPut]["/api/skills/:skillId"] = wrapAction(signed, controller.UpdateSkill)
	r[http.MethodDelete]["/api/skills/:skillId"] = wrapAction(signed, controller.DeleteSkill)
}
