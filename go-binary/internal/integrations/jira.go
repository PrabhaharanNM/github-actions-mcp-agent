package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// CreateJiraTicket creates a Jira issue for the failed build and returns the ticket key.
func CreateJiraTicket(req *models.AnalysisRequest, analysis *models.ClaudeAnalysis, teamMgr *models.TeamManager, ctx *models.BuildContext) (string, error) {
	if req.Jira.Url == "" || req.Jira.Project == "" {
		log.Println("[JIRA] Jira URL or project not configured, skipping ticket creation")
		return "", nil
	}

	jiraURL := strings.TrimRight(req.Jira.Url, "/")

	// Build summary (max 255 chars)
	summary := fmt.Sprintf("Build Failed: %s/%s #%d - %s", ctx.Owner, ctx.Repo, ctx.RunNumber, analysis.Category)
	if len(summary) > 255 {
		summary = summary[:255]
	}

	// Determine issue type and optional parent
	issueTypeName := "Bug"
	if req.Jira.EpicKey != "" {
		issueTypeName = "Sub-task"
	}

	// Build the request body
	fields := map[string]interface{}{
		"project":     map[string]string{"key": req.Jira.Project},
		"issuetype":   map[string]string{"name": issueTypeName},
		"summary":     summary,
		"description": convertToJiraMarkup(analysis, ctx),
	}

	if req.Jira.EpicKey != "" {
		fields["parent"] = map[string]string{"key": req.Jira.EpicKey}
	}

	if teamMgr != nil && teamMgr.JiraUsername != "" {
		fields["assignee"] = map[string]string{"name": teamMgr.JiraUsername}
	}

	body := map[string]interface{}{
		"fields": fields,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		log.Printf("[JIRA] Failed to marshal request body: %v", err)
		return "", fmt.Errorf("failed to marshal jira request: %w", err)
	}

	// Create HTTP request
	apiURL := fmt.Sprintf("%s/rest/api/2/issue", jiraURL)
	httpReq, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("[JIRA] Failed to create HTTP request: %v", err)
		return "", fmt.Errorf("failed to create jira http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(req.Jira.Username, req.Jira.ApiToken)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[JIRA] HTTP request failed: %v", err)
		return "", fmt.Errorf("jira http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[JIRA] Failed to read response body: %v", err)
		return "", fmt.Errorf("failed to read jira response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[JIRA] API returned status %d: %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("jira API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response for the issue key
	var result struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("[JIRA] Failed to parse response: %v", err)
		return "", fmt.Errorf("failed to parse jira response: %w", err)
	}

	log.Printf("[JIRA] Created ticket: %s", result.Key)
	return result.Key, nil
}

// convertToJiraMarkup converts the Claude analysis into Jira wiki markup format.
func convertToJiraMarkup(analysis *models.ClaudeAnalysis, ctx *models.BuildContext) string {
	var sb strings.Builder

	// Root Cause Summary
	sb.WriteString("h2. Root Cause Summary\n")
	sb.WriteString(analysis.RootCauseSummary)
	sb.WriteString("\n\n")

	if analysis.RootCauseDetails != "" {
		sb.WriteString(analysis.RootCauseDetails)
		sb.WriteString("\n\n")
	}

	// Evidence
	if len(analysis.Evidence) > 0 {
		sb.WriteString("h2. Evidence\n")
		for _, e := range analysis.Evidence {
			sb.WriteString(fmt.Sprintf("* %s\n", e))
		}
		sb.WriteString("\n")
	}

	// Next Steps
	if len(analysis.NextSteps) > 0 {
		sb.WriteString("h2. Next Steps\n")
		for i, step := range analysis.NextSteps {
			sb.WriteString(fmt.Sprintf("# %d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	// Build Info
	sb.WriteString("h2. Build Info\n")
	sb.WriteString(fmt.Sprintf("* *Repository:* %s/%s\n", ctx.Owner, ctx.Repo))
	sb.WriteString(fmt.Sprintf("* *Workflow:* %s\n", ctx.Workflow))
	sb.WriteString(fmt.Sprintf("* *Run:* #%d (ID: %d)\n", ctx.RunNumber, ctx.RunID))
	sb.WriteString(fmt.Sprintf("* *Ref:* %s\n", ctx.Ref))
	sb.WriteString(fmt.Sprintf("* *SHA:* %s\n", ctx.SHA))
	sb.WriteString(fmt.Sprintf("* *Actor:* %s\n", ctx.Actor))
	if ctx.FailedStep != "" {
		sb.WriteString(fmt.Sprintf("* *Failed Step:* %s\n", ctx.FailedStep))
	}
	if ctx.FailedJob != "" {
		sb.WriteString(fmt.Sprintf("* *Failed Job:* %s\n", ctx.FailedJob))
	}
	sb.WriteString(fmt.Sprintf("* *Confidence:* %s\n", analysis.Confidence))

	return sb.String()
}
