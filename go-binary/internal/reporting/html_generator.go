package reporting

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Workflow Failure Analysis Report</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f6f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;color:#212529;line-height:1.6;">

<div style="max-width:800px;margin:0 auto;padding:20px;">

  <!-- Header -->
  <div style="background-color:#ffffff;border-top:4px solid #dc3545;border-radius:4px;padding:24px 30px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
    <h1 style="margin:0;font-size:24px;font-weight:600;color:#212529;">Workflow Failure Analysis Report</h1>
    <p style="margin:6px 0 0 0;font-size:14px;color:#6c757d;">Automated Root Cause Analysis &mdash; GitHub Actions</p>
  </div>

  <!-- Summary Card -->
  <div style="background-color:#ffffff;border-radius:4px;padding:24px 30px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
    <h2 style="margin:0 0 16px 0;font-size:18px;font-weight:600;color:#212529;border-bottom:1px solid #e9ecef;padding-bottom:10px;">Summary</h2>

    {{if .Category}}
    <div style="margin-bottom:14px;">
      <span style="font-size:13px;font-weight:600;color:#6c757d;text-transform:uppercase;letter-spacing:0.5px;">Category</span><br>
      <span style="display:inline-block;margin-top:4px;padding:4px 12px;border-radius:12px;font-size:13px;font-weight:600;color:#ffffff;background-color:{{.CategoryColor}};">{{.Category}}</span>
    </div>
    {{end}}

    {{if .RootCauseSummary}}
    <div style="margin-bottom:14px;">
      <span style="font-size:13px;font-weight:600;color:#6c757d;text-transform:uppercase;letter-spacing:0.5px;">Root Cause</span>
      <p style="margin:4px 0 0 0;font-size:15px;color:#212529;">{{.RootCauseSummary}}</p>
    </div>
    {{end}}

    {{if .TeamName}}
    <div style="margin-bottom:14px;">
      <span style="font-size:13px;font-weight:600;color:#6c757d;text-transform:uppercase;letter-spacing:0.5px;">Responsible Team</span>
      <p style="margin:4px 0 0 0;font-size:15px;color:#212529;">{{.TeamName}}{{if .TeamEmail}} &mdash; <a href="mailto:{{.TeamEmail}}" style="color:#17a2b8;text-decoration:none;">{{.TeamEmail}}</a>{{end}}</p>
    </div>
    {{end}}

    {{if .Confidence}}
    <div style="margin-bottom:4px;">
      <span style="font-size:13px;font-weight:600;color:#6c757d;text-transform:uppercase;letter-spacing:0.5px;">Analysis Confidence</span>
      <p style="margin:4px 0 0 0;font-size:15px;color:#212529;">{{.Confidence}}</p>
    </div>
    {{end}}
  </div>

  <!-- Build Information Card -->
  <div style="background-color:#ffffff;border-radius:4px;padding:24px 30px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
    <h2 style="margin:0 0 16px 0;font-size:18px;font-weight:600;color:#212529;border-bottom:1px solid #e9ecef;padding-bottom:10px;">Workflow Information</h2>
    <table style="width:100%;border-collapse:collapse;font-size:14px;">
      {{if .Repository}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Repository</td>
        <td style="padding:6px 0;color:#212529;">{{.Repository}}</td>
      </tr>
      {{end}}
      {{if .Workflow}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Workflow</td>
        <td style="padding:6px 0;color:#212529;">{{.Workflow}}</td>
      </tr>
      {{end}}
      {{if .RunNumber}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Run Number</td>
        <td style="padding:6px 0;color:#212529;">#{{.RunNumber}}</td>
      </tr>
      {{end}}
      {{if .Ref}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Ref</td>
        <td style="padding:6px 0;color:#212529;">{{.Ref}}</td>
      </tr>
      {{end}}
      {{if .CommitSHA}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Commit</td>
        <td style="padding:6px 0;color:#212529;"><code style="background-color:#f1f3f5;padding:2px 6px;border-radius:3px;font-family:'SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace;font-size:13px;">{{.CommitSHA}}</code></td>
      </tr>
      {{end}}
      {{if .Actor}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Triggered By</td>
        <td style="padding:6px 0;color:#212529;">{{.Actor}}</td>
      </tr>
      {{end}}
      {{if .FailedJob}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Failed Job</td>
        <td style="padding:6px 0;color:#dc3545;font-weight:600;">{{.FailedJob}}</td>
      </tr>
      {{end}}
      {{if .FailedStep}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Failed Step</td>
        <td style="padding:6px 0;color:#dc3545;font-weight:600;">{{.FailedStep}}</td>
      </tr>
      {{end}}
      {{if .PrLink}}
      <tr>
        <td style="padding:6px 12px 6px 0;font-weight:600;color:#6c757d;white-space:nowrap;vertical-align:top;width:140px;">Pull Request</td>
        <td style="padding:6px 0;"><a href="{{.PrLink}}" style="color:#17a2b8;text-decoration:none;">{{.PrLink}}</a></td>
      </tr>
      {{end}}
    </table>
  </div>

  <!-- Root Cause Details -->
  {{if .RootCauseDetails}}
  <div style="background-color:#ffffff;border-radius:4px;padding:24px 30px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
    <h2 style="margin:0 0 16px 0;font-size:18px;font-weight:600;color:#212529;border-bottom:1px solid #e9ecef;padding-bottom:10px;">Root Cause Details</h2>
    {{range .RootCauseDetailsParagraphs}}
    <p style="margin:0 0 12px 0;font-size:14px;color:#212529;line-height:1.7;">{{.}}</p>
    {{end}}
  </div>
  {{end}}

  <!-- Evidence -->
  {{if .Evidence}}
  <div style="background-color:#ffffff;border-radius:4px;padding:24px 30px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
    <h2 style="margin:0 0 16px 0;font-size:18px;font-weight:600;color:#212529;border-bottom:1px solid #e9ecef;padding-bottom:10px;">Evidence</h2>
    <ul style="margin:0;padding:0 0 0 20px;">
      {{range .Evidence}}
      <li style="margin-bottom:10px;font-size:14px;color:#212529;">{{.}}</li>
      {{end}}
    </ul>
  </div>
  {{end}}

  <!-- Log Excerpts -->
  {{if .LogExcerpts}}
  <div style="background-color:#ffffff;border-radius:4px;padding:24px 30px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
    <h2 style="margin:0 0 16px 0;font-size:18px;font-weight:600;color:#212529;border-bottom:1px solid #e9ecef;padding-bottom:10px;">Log Excerpts</h2>
    {{range .LogExcerpts}}
    <pre style="background-color:#f1f3f5;border:1px solid #dee2e6;border-radius:4px;padding:14px;margin:0 0 12px 0;overflow-x:auto;white-space:pre-wrap;word-wrap:break-word;"><code style="font-family:'SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace;font-size:13px;color:#212529;">{{.}}</code></pre>
    {{end}}
  </div>
  {{end}}

  <!-- Next Steps -->
  {{if .NextSteps}}
  <div style="background-color:#ffffff;border-radius:4px;padding:24px 30px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
    <h2 style="margin:0 0 16px 0;font-size:18px;font-weight:600;color:#212529;border-bottom:1px solid #e9ecef;padding-bottom:10px;">Next Steps</h2>
    <ol style="margin:0;padding:0 0 0 20px;">
      {{range .NextSteps}}
      <li style="margin-bottom:8px;font-size:14px;color:#212529;">{{.}}</li>
      {{end}}
    </ol>
  </div>
  {{end}}

  <!-- Footer -->
  <div style="text-align:center;padding:20px 0;font-size:12px;color:#6c757d;">
    <p style="margin:0 0 4px 0;">Generated by <strong>MCP Agent Action</strong> &mdash; {{.Timestamp}}</p>
    {{if .RunUrl}}
    <p style="margin:4px 0 0 0;">Workflow Run: <a href="{{.RunUrl}}" style="color:#17a2b8;text-decoration:none;">{{.RunUrl}}</a></p>
    {{end}}
    {{if .JiraTicketUrl}}
    <p style="margin:4px 0 0 0;">Jira Ticket: <a href="{{.JiraTicketUrl}}" style="color:#17a2b8;text-decoration:none;">{{.JiraTicketKey}}</a></p>
    {{end}}
  </div>

</div>
</body>
</html>`

// htmlReportData is the view model passed into the HTML template.
type htmlReportData struct {
	Category                   string
	CategoryColor              string
	RootCauseSummary           string
	TeamName                   string
	TeamEmail                  string
	Confidence                 string
	Repository                 string
	Workflow                   string
	RunNumber                  int
	Ref                        string
	CommitSHA                  string
	Actor                      string
	FailedJob                  string
	FailedStep                 string
	PrLink                     string
	RunUrl                     string
	RootCauseDetails           string
	RootCauseDetailsParagraphs []string
	Evidence                   []string
	LogExcerpts                []string
	NextSteps                  []string
	Timestamp                  string
	JiraTicketKey              string
	JiraTicketUrl              string
}

// GenerateHTML produces a self-contained HTML report from the analysis results,
// build context, and team manager information. The report uses inline CSS for
// email compatibility and renders safely via html/template.
func GenerateHTML(analysis *models.ClaudeAnalysis, buildCtx *models.BuildContext, teamMgr *models.TeamManager) string {
	data := buildReportData(analysis, buildCtx, teamMgr)

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Sprintf("<html><body><p>Error generating report: %s</p></body></html>", err.Error())
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("<html><body><p>Error generating report: %s</p></body></html>", err.Error())
	}

	return buf.String()
}

// buildReportData maps the domain models into the flat view model consumed by
// the HTML template.
func buildReportData(analysis *models.ClaudeAnalysis, buildCtx *models.BuildContext, teamMgr *models.TeamManager) htmlReportData {
	data := htmlReportData{
		Timestamp: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	}

	// Populate from ClaudeAnalysis.
	if analysis != nil {
		data.Category = analysis.Category
		data.CategoryColor = categoryColor(analysis.Category)
		data.RootCauseSummary = analysis.RootCauseSummary
		data.RootCauseDetails = analysis.RootCauseDetails
		data.Confidence = analysis.Confidence

		// Split details into paragraphs for cleaner rendering.
		if analysis.RootCauseDetails != "" {
			data.RootCauseDetailsParagraphs = splitParagraphs(analysis.RootCauseDetails)
		}

		// Separate plain evidence from log excerpts (lines that look like log output).
		for _, item := range analysis.Evidence {
			if looksLikeLogExcerpt(item) {
				data.LogExcerpts = append(data.LogExcerpts, item)
			} else {
				data.Evidence = append(data.Evidence, item)
			}
		}

		data.NextSteps = analysis.NextSteps
	}

	// Populate from BuildContext.
	if buildCtx != nil {
		data.Repository = buildCtx.Owner + "/" + buildCtx.Repo
		data.Workflow = buildCtx.Workflow
		data.RunNumber = buildCtx.RunNumber
		data.Ref = buildCtx.Ref
		data.CommitSHA = buildCtx.SHA
		data.Actor = buildCtx.Actor
		data.FailedJob = buildCtx.FailedJob
		data.FailedStep = buildCtx.FailedStep

		// Build PR link if applicable.
		if buildCtx.PullRequestNumber > 0 && buildCtx.Owner != "" && buildCtx.Repo != "" {
			data.PrLink = fmt.Sprintf("https://github.com/%s/%s/pull/%d", buildCtx.Owner, buildCtx.Repo, buildCtx.PullRequestNumber)
		}

		// Build workflow run URL.
		if buildCtx.RunID > 0 && buildCtx.Owner != "" && buildCtx.Repo != "" {
			data.RunUrl = fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", buildCtx.Owner, buildCtx.Repo, buildCtx.RunID)
		}
	}

	// Populate from TeamManager.
	if teamMgr != nil {
		data.TeamName = teamMgr.Name
		data.TeamEmail = teamMgr.Email
	}

	return data
}

// categoryColor returns the badge background color based on the failure category.
func categoryColor(category string) string {
	cat := strings.ToLower(category)
	switch {
	case strings.Contains(cat, "infrastructure"), strings.Contains(cat, "infra"):
		return "#ffc107" // warning yellow
	case strings.Contains(cat, "dependency"):
		return "#fd7e14" // orange
	case strings.Contains(cat, "test"):
		return "#17a2b8" // info blue
	case strings.Contains(cat, "configuration"), strings.Contains(cat, "config"):
		return "#6f42c1" // purple
	case strings.Contains(cat, "unknown"):
		return "#6c757d" // gray
	default:
		// Covers code changes, compilation errors, build failures, etc.
		return "#dc3545" // danger red
	}
}

// splitParagraphs splits a block of text into non-empty paragraphs by double
// newlines or single newlines.
func splitParagraphs(text string) []string {
	// First try splitting on double newlines.
	parts := strings.Split(text, "\n\n")
	var paragraphs []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	if len(paragraphs) > 0 {
		return paragraphs
	}
	// Fallback: return entire text as one paragraph.
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		return []string{trimmed}
	}
	return nil
}

// looksLikeLogExcerpt applies simple heuristics to decide whether an evidence
// string is a log excerpt (and should be rendered in a <pre><code> block)
// rather than a plain-text bullet point.
func looksLikeLogExcerpt(s string) bool {
	if strings.Contains(s, "\n") {
		return true
	}
	indicators := []string{"Exception", "Error:", "at ", "Caused by:", "FATAL", "FAILED", "exit code"}
	lower := strings.ToLower(s)
	for _, ind := range indicators {
		if strings.Contains(lower, strings.ToLower(ind)) && len(s) > 80 {
			return true
		}
	}
	return false
}
