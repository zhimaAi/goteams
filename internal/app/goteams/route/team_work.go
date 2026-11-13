// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"net/http"

	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
)

// RegTeamWorkRoutes registers team work routes.
func RegTeamWorkRoutes(r lib_web.Route) {
	teamWorkPermission := withAnyPerm(
		"team_work:view_today",
		"team_work:view_yesterday",
		"team_work:view_tomorrow",
		"team_work:view_custom_date",
		"team_work:view_unscheduled",
		"team_work:view_scheduled",
	)
	r[http.MethodGet][`/api/team-work`] = wrapAction(teamWorkPermission, controller.ListTeamWork)
	r[http.MethodGet][`/api/team-work/members`] = wrapAction(teamWorkPermission, controller.ListTeamWorkMembers)
	r[http.MethodGet][`/api/team-work/member-candidates`] = wrapAction(teamWorkPermission, controller.ListTeamWorkMemberCandidates)
	r[http.MethodPut][`/api/team-work/members`] = wrapAction(teamWorkPermission, controller.UpdateTeamWorkMembers)
}
