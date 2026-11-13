package controller

import "goteams/internal/app/goteams/business"

type dashboardSLARiskFilterRequest struct {
	From       int64 `json:"from"`
	To         int64 `json:"to"`
	AssigneeId int   `json:"assignee_id"`
	PriorityId *int  `json:"priority_id"`
	Page       int   `json:"page"`
	Size       int   `json:"size"`
}

type dashboardSLADistributionBucketResponse struct {
	StageId   int    `json:"stage_id,omitempty"`
	StageName string `json:"stage_name,omitempty"`
	Key       string `json:"key"`
	Count     int64  `json:"count"`
}

type dashboardSLADistributionResponse struct {
	Enabled     bool                                     `json:"enabled"`
	EvaluatedAt int64                                    `json:"evaluated_at"`
	Total       int64                                    `json:"total"`
	Buckets     []dashboardSLADistributionBucketResponse `json:"buckets"`
}

type dashboardSLARiskRowResponse struct {
	StageTimes       []*int64 `json:"stage_times"`
	Id               int64    `json:"id"`
	Title            string   `json:"title"`
	PriorityId       int      `json:"priority_id"`
	StatusId         int      `json:"status_id"`
	AssigneeIds      []int    `json:"assignee_ids"`
	StageId          int64    `json:"stage_id"`
	StageName        string   `json:"stage_name"`
	StageState       string   `json:"stage_state"`
	CalculationState string   `json:"calculation_state"`
	ElapsedSeconds   *int64   `json:"elapsed_seconds"`
	Risk             string   `json:"risk"`
	Seconds          int64    `json:"seconds"`

	StageResults []*dashboardSLARiskStageResultResponse `json:"stage_results"`
}

type dashboardSLARiskStageResultResponse struct {
	State            string `json:"state"`
	Reason           string `json:"reason,omitempty"`
	CalculationState string `json:"calculation_state"`
	Risk             string `json:"risk,omitempty"`
	ElapsedSeconds   *int64 `json:"elapsed_seconds"`
	RemainingSeconds *int64 `json:"remaining_seconds"`
	OverdueSeconds   *int64 `json:"overdue_seconds"`
}

type dashboardSLARiskStageResponse struct {
	Id               int    `json:"id"`
	Name             string `json:"name"`
	Critical         int64  `json:"critical"`
	ThresholdSeconds int    `json:"threshold_seconds"`
}

type dashboardSLARiskResponse struct {
	Stages           []dashboardSLARiskStageResponse `json:"stages"`
	Enabled          bool                            `json:"enabled"`
	EvaluatedAt      int64                           `json:"evaluated_at"`
	Total            int64                           `json:"total"`
	Overdue          int64                           `json:"overdue"`
	Started          int64                           `json:"started"`
	OverdueRate      *float64                        `json:"overdue_rate"`
	Page             int                             `json:"page"`
	Size             int                             `json:"size"`
	ThresholdSeconds int                             `json:"threshold_seconds"`
	List             []dashboardSLARiskRowResponse   `json:"list"`
}

func toDashboardSLADistributionResponse(result *business.DashboardSLADistributionModel) *dashboardSLADistributionResponse {
	if result == nil {
		return nil
	}
	buckets := make([]dashboardSLADistributionBucketResponse, 0, len(result.Buckets))
	for _, item := range result.Buckets {
		buckets = append(buckets, dashboardSLADistributionBucketResponse{Key: item.Key, StageId: item.StageId, StageName: item.StageName, Count: item.Count})
	}
	return &dashboardSLADistributionResponse{Enabled: result.Enabled, EvaluatedAt: result.EvaluatedAt, Total: result.Total, Buckets: buckets}
}

func toDashboardSLARiskResponse(result *business.DashboardSLARiskModel) *dashboardSLARiskResponse {
	if result == nil {
		return nil
	}
	list := make([]dashboardSLARiskRowResponse, 0, len(result.Rows))
	for _, item := range result.Rows {
		stageResults := make([]*dashboardSLARiskStageResultResponse, len(item.StageResults))
		for index, stage := range item.StageResults {
			if stage.SLAStageResultModel != nil {
				stageResults[index] = &dashboardSLARiskStageResultResponse{
					State: stage.State, Reason: stage.Reason, CalculationState: stage.CalculationState, Risk: stage.Risk,
					ElapsedSeconds: stage.ElapsedSeconds, RemainingSeconds: stage.RemainingSeconds, OverdueSeconds: stage.OverdueSeconds,
				}
			}
		}
		list = append(list, dashboardSLARiskRowResponse{
			Id: item.Id, Title: item.Title, PriorityId: item.PriorityId, StatusId: item.StatusId, AssigneeIds: item.AssigneeIds,
			StageId: item.Stage.StageId, StageName: item.Stage.StageName, StageState: item.Stage.State, CalculationState: item.Stage.CalculationState,
			ElapsedSeconds: item.Stage.ElapsedSeconds, StageTimes: item.StageTimes, StageResults: stageResults,
			Risk: item.Risk, Seconds: item.Seconds,
		})
	}
	stages := make([]dashboardSLARiskStageResponse, 0, len(result.Stages))
	for _, stage := range result.Stages {
		stages = append(stages, dashboardSLARiskStageResponse{
			Id: stage.Id, Name: stage.Name, Critical: stage.Critical, ThresholdSeconds: stage.ThresholdSeconds,
		})
	}
	return &dashboardSLARiskResponse{
		Stages:  stages,
		Enabled: result.Enabled, EvaluatedAt: result.EvaluatedAt, Total: result.Total,
		Overdue: result.Overdue, Started: result.Started,
		OverdueRate: result.OverdueRate, Page: result.Page, Size: result.Size, ThresholdSeconds: result.ThresholdSeconds, List: list,
	}
}
