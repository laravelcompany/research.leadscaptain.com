package domain

import "time"

type Objective struct {
	ID int64 `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Status string `json:"status"`
	Priority int `json:"priority"`
	TargetLeads *int `json:"target_leads"`
	MinimumScore int `json:"minimum_score"`
	MaxIterations int `json:"max_iterations"`
	MaxTasks int `json:"max_tasks"`
	IterationCount int `json:"iteration_count"`
	StartedAt *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	AllowedTools string `json:"allowed_tools"`
	ConfigJSON string `json:"config_json"`
}
type AgentRun struct {
	ID int64 `json:"id"`
	ObjectiveID int64 `json:"objective_id"`
	Status string `json:"status"`
	Iteration int `json:"iteration"`
	StartedAt *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	TotalTasks int `json:"total_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	FailedTasks int `json:"failed_tasks"`
	AIRequests int `json:"ai_requests"`
	APIRequests int `json:"api_requests"`
}
type Task struct {
	ID int64 `json:"id"`
	ObjectiveID int64 `json:"objective_id"`
	RunID *int64 `json:"run_id"`
	ParentTaskID *int64 `json:"parent_task_id"`
	Title string `json:"title"`
	Description string `json:"description"`
	Category string `json:"category"`
	Status string `json:"status"`
	Priority int `json:"priority"`
	InputJSON string `json:"input_json"`
	OutputJSON string `json:"output_json"`
	PlannedSteps string `json:"planned_steps"`
	Attempts int `json:"attempts"`
	MaxAttempts int `json:"max_attempts"`
	StartedAt *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Error *string `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Lead struct {
	ID int64 `json:"id"`
	ExternalKey *string `json:"external_key"`
	FirstName *string `json:"first_name"`
	LastName *string `json:"last_name"`
	FullName *string `json:"full_name"`
	Email *string `json:"email"`
	EmailStatus *string `json:"email_status"`
	CompanyName *string `json:"company_name"`
	CompanyDomain *string `json:"company_domain"`
	PositionTitle *string `json:"position_title"`
	Department *string `json:"department"`
	IndustryName *string `json:"industry_name"`
	CountryCode *string `json:"country_code"`
	CountryName *string `json:"country_name"`
	City *string `json:"city"`
	LinkedinURL *string `json:"linkedin_url"`
	WebsiteURL *string `json:"website_url"`
	LeadScore int `json:"lead_score"`
	ScoreBreakdown *string `json:"score_breakdown"`
	Source *string `json:"source"`
	RawData *string `json:"raw_data"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
