package models

// AnalysisResult represents the output of a build failure analysis.
type AnalysisResult struct {
	Status           string `json:"status"`
	Category         string `json:"category"`
	RootCauseSummary string `json:"rootCauseSummary"`
	RootCauseDetails string `json:"rootCauseDetails"`
	ResponsibleTeam  string `json:"responsibleTeam"`
	TeamEmail        string `json:"teamEmail"`
	HtmlReport       string `json:"htmlReport"`

	JiraTicketKey  string `json:"jiraTicketKey"`
	JiraTicketUrl  string `json:"jiraTicketUrl"`
	GithubIssueUrl string `json:"githubIssueUrl"`

	Evidence  []string `json:"evidence"`
	NextSteps []string `json:"nextSteps"`

	ErrorMessage string `json:"errorMessage"`

	AnalysisTimeMs int64 `json:"analysisTimeMs"`

	ClaudeAnalysis ClaudeAnalysis `json:"claudeAnalysis"`
}

// ClaudeAnalysis represents the structured output from the Claude model.
type ClaudeAnalysis struct {
	Category         string   `json:"category"`
	RootCauseSummary string   `json:"rootCauseSummary"`
	RootCauseDetails string   `json:"rootCauseDetails"`
	Evidence         []string `json:"evidence"`
	NextSteps        []string `json:"nextSteps"`
	Confidence       string   `json:"confidence"`
}
