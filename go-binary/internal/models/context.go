package models

// BuildContext represents the contextual information about a build failure.
type BuildContext struct {
	Owner             string `json:"owner"`
	Repo              string `json:"repo"`
	Workflow          string `json:"workflow"`
	Job               string `json:"job"`
	Ref               string `json:"ref"`
	SHA               string `json:"sha"`
	Actor             string `json:"actor"`
	EventName         string `json:"event_name"`
	RunID             int64  `json:"run_id"`
	RunNumber         int    `json:"run_number"`
	PullRequestNumber int    `json:"pull_request_number"`

	FailedStep           string   `json:"failed_step"`
	FailedJob            string   `json:"failed_job"`
	ErrorMessages        []string `json:"error_messages"`
	FailedTests          []string `json:"failed_tests"`
	SuspectedRepository  string   `json:"suspected_repository"`
	AllJobs              []string `json:"all_jobs"`
	ConsoleLog           string   `json:"console_log"`
}
