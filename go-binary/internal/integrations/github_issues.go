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

// CreateGithubIssue creates a GitHub issue for the failed build and returns the issue URL.
func CreateGithubIssue(req *models.AnalysisRequest, analysis *models.ClaudeAnalysis, ctx *models.BuildContext, team *models.TeamManager) (string, error) {
	if req.GithubToken == "" {
		log.Println("[GITHUB] GitHub token not configured, skipping issue creation")
		return "", nil
	}

	apiURL := strings.TrimRight(req.ApiUrl, "/")

	// Build the issue title
	title := fmt.Sprintf("Build Failure: %s/%s #%d - %s", ctx.Owner, ctx.Repo, ctx.RunNumber, analysis.Category)
	if len(title) > 256 {
		title = title[:256]
	}

	// Build the issue body in markdown
	body := buildMarkdownBody(analysis, ctx, team, req)

	// Build labels
	labels := []string{"mcp-analysis", analysis.Category}

	// Build the request payload
	payload := map[string]interface{}{
		"title":  title,
		"body":   body,
		"labels": labels,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[GITHUB] Failed to marshal request body: %v", err)
		return "", fmt.Errorf("failed to marshal github issue request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/repos/%s/%s/issues", apiURL, ctx.Owner, ctx.Repo)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("[GITHUB] Failed to create HTTP request: %v", err)
		return "", fmt.Errorf("failed to create github http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("Authorization", "Bearer "+req.GithubToken)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[GITHUB] HTTP request failed: %v", err)
		return "", fmt.Errorf("github http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[GITHUB] Failed to read response body: %v", err)
		return "", fmt.Errorf("failed to read github response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[GITHUB] API returned status %d: %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("github API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response for issue URL
	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("[GITHUB] Failed to parse response: %v", err)
		return "", fmt.Errorf("failed to parse github response: %w", err)
	}

	log.Printf("[GITHUB] Created issue: %s", result.HTMLURL)
	return result.HTMLURL, nil
}

// CommentOnPR posts a comment on the pull request associated with the build failure.
func CommentOnPR(req *models.AnalysisRequest, analysis *models.ClaudeAnalysis, ctx *models.BuildContext, team *models.TeamManager) error {
	if req.GithubToken == "" {
		log.Println("[GITHUB] GitHub token not configured, skipping PR comment")
		return nil
	}

	if ctx.PullRequestNumber == 0 {
		log.Println("[GITHUB] No pull request number available, skipping PR comment")
		return nil
	}

	apiURL := strings.TrimRight(req.ApiUrl, "/")

	// Build the comment body in markdown
	body := buildMarkdownBody(analysis, ctx, team, req)

	// Build the request payload
	payload := map[string]interface{}{
		"body": body,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[GITHUB] Failed to marshal comment body: %v", err)
		return fmt.Errorf("failed to marshal pr comment request: %w", err)
	}

	// Create HTTP request (GitHub Issues API handles PR comments)
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", apiURL, ctx.Owner, ctx.Repo, ctx.PullRequestNumber)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("[GITHUB] Failed to create HTTP request: %v", err)
		return fmt.Errorf("failed to create github http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("Authorization", "Bearer "+req.GithubToken)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[GITHUB] HTTP request failed: %v", err)
		return fmt.Errorf("github http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[GITHUB] Failed to read response body: %v", err)
		return fmt.Errorf("failed to read github response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[GITHUB] API returned status %d: %s", resp.StatusCode, string(respBody))
		return fmt.Errorf("github API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[GITHUB] Comment posted on PR #%d in %s/%s", ctx.PullRequestNumber, ctx.Owner, ctx.Repo)
	return nil
}

// buildMarkdownBody constructs a markdown-formatted body for GitHub issues and PR comments.
func buildMarkdownBody(analysis *models.ClaudeAnalysis, ctx *models.BuildContext, team *models.TeamManager, req *models.AnalysisRequest) string {
	var sb strings.Builder

	sb.WriteString("## MCP Build Failure Analysis\n\n")

	// Category badge
	sb.WriteString(fmt.Sprintf("**Category:** `%s`\n", analysis.Category))
	sb.WriteString(fmt.Sprintf("**Confidence:** %s\n\n", analysis.Confidence))

	// Root Cause Summary
	sb.WriteString("### Root Cause Summary\n\n")
	sb.WriteString(analysis.RootCauseSummary)
	sb.WriteString("\n\n")

	if analysis.RootCauseDetails != "" {
		sb.WriteString(analysis.RootCauseDetails)
		sb.WriteString("\n\n")
	}

	// Evidence
	if len(analysis.Evidence) > 0 {
		sb.WriteString("### Evidence\n\n")
		for _, e := range analysis.Evidence {
			sb.WriteString(fmt.Sprintf("- %s\n", e))
		}
		sb.WriteString("\n")
	}

	// Next Steps
	if len(analysis.NextSteps) > 0 {
		sb.WriteString("### Next Steps\n\n")
		for i, step := range analysis.NextSteps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	// Team Info
	if team != nil {
		sb.WriteString("### Responsible Team\n\n")
		sb.WriteString(fmt.Sprintf("- **Name:** %s\n", team.Name))
		if team.Email != "" {
			sb.WriteString(fmt.Sprintf("- **Email:** %s\n", team.Email))
		}
		sb.WriteString("\n")
	}

	// Build Info
	sb.WriteString("### Build Info\n\n")
	sb.WriteString(fmt.Sprintf("- **Repository:** %s/%s\n", ctx.Owner, ctx.Repo))
	sb.WriteString(fmt.Sprintf("- **Workflow:** %s\n", ctx.Workflow))
	if ctx.Job != "" {
		sb.WriteString(fmt.Sprintf("- **Job:** %s\n", ctx.Job))
	}
	sb.WriteString(fmt.Sprintf("- **Run:** #%d (ID: %d)\n", ctx.RunNumber, ctx.RunID))
	sb.WriteString(fmt.Sprintf("- **Ref:** %s\n", ctx.Ref))
	sb.WriteString(fmt.Sprintf("- **SHA:** `%s`\n", ctx.SHA))
	sb.WriteString(fmt.Sprintf("- **Actor:** %s\n", ctx.Actor))
	if ctx.FailedStep != "" {
		sb.WriteString(fmt.Sprintf("- **Failed Step:** %s\n", ctx.FailedStep))
	}
	if ctx.FailedJob != "" {
		sb.WriteString(fmt.Sprintf("- **Failed Job:** %s\n", ctx.FailedJob))
	}

	// Link to workflow run
	serverURL := strings.TrimRight(req.ServerUrl, "/")
	if serverURL != "" {
		workflowURL := fmt.Sprintf("%s/%s/%s/actions/runs/%d", serverURL, ctx.Owner, ctx.Repo, ctx.RunID)
		sb.WriteString(fmt.Sprintf("\n[View Workflow Run](%s)\n", workflowURL))
	}

	sb.WriteString("\n---\n*Generated by MCP Build Failure Agent*\n")

	return sb.String()
}
