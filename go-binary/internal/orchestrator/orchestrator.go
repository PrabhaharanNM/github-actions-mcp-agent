package orchestrator

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/agents"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/claude"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/correlation"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/integrations"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/parser"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/reporting"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/team"
)

// Analyze runs the full build-failure root cause analysis pipeline.
func Analyze(ctx context.Context, req *models.AnalysisRequest) (*models.AnalysisResult, error) {
	start := time.Now()

	// Step 1: Fetch workflow run logs from GitHub API.
	rawLogs, err := fetchWorkflowLogs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fetch workflow logs: %w", err)
	}

	// Step 2: Parse the raw logs into structured build context.
	buildCtx := parser.Parse(rawLogs)

	// Populate build context with request metadata.
	buildCtx.Owner = req.Owner
	buildCtx.Repo = req.Repo
	buildCtx.Workflow = req.Workflow
	buildCtx.Job = req.Job
	buildCtx.Ref = req.Ref
	buildCtx.SHA = req.SHA
	buildCtx.Actor = req.Actor
	buildCtx.EventName = req.EventName
	buildCtx.RunID = req.RunID
	buildCtx.RunNumber = req.RunNumber
	buildCtx.PullRequestNumber = req.PullRequestNumber

	// Step 3: Run MCP agents in parallel (GitHub, Kubernetes, JFrog).
	mcpResults := runAgentsParallel(ctx, req, buildCtx)

	// Step 4: Cross-correlate findings from all agents.
	correlated := correlation.Analyze(buildCtx, mcpResults)

	// Step 5: AI root cause analysis via Claude.
	claudeResult, err := claude.Analyze(ctx, req, buildCtx, mcpResults, correlated)
	if err != nil {
		return nil, fmt.Errorf("claude analysis: %w", err)
	}

	// Step 6: Assign responsible team.
	teamAssignment := team.Assign(req, buildCtx, correlated)

	// Step 7: Generate HTML report.
	htmlReport := reporting.GenerateHTML(claudeResult, buildCtx, teamAssignment)

	// Step 8: Conditional integrations.
	var githubIssueURL string
	var jiraTicketKey string

	if req.CreateIssue {
		issueURL, err := integrations.CreateGithubIssue(req, claudeResult, buildCtx, teamAssignment)
		if err != nil {
			fmt.Printf("warning: failed to create GitHub issue: %v\n", err)
		} else {
			githubIssueURL = issueURL
		}
	}

	if req.CommentOnPr && req.PullRequestNumber > 0 {
		go func() {
			if err := integrations.CommentOnPR(req, claudeResult, buildCtx, teamAssignment); err != nil {
				fmt.Printf("warning: failed to comment on PR: %v\n", err)
			}
		}()
	}

	if req.CreateJiraTicket {
		key, err := integrations.CreateJiraTicket(req, claudeResult, teamAssignment, buildCtx)
		if err != nil {
			fmt.Printf("warning: failed to create Jira ticket: %v\n", err)
		} else {
			jiraTicketKey = key
		}
	}

	if req.SendEmail {
		go func() {
			if err := integrations.SendEmail(req, claudeResult, teamAssignment, htmlReport, buildCtx); err != nil {
				fmt.Printf("warning: failed to send email: %v\n", err)
			}
		}()
	}

	go func() {
		if err := integrations.TrackMTTR(req, claudeResult, teamAssignment, buildCtx); err != nil {
			fmt.Printf("warning: failed to track MTTR: %v\n", err)
		}
	}()

	// Build Jira URL from key.
	var jiraTicketUrl string
	if jiraTicketKey != "" && req.Jira.Url != "" {
		jiraTicketUrl = strings.TrimRight(req.Jira.Url, "/") + "/browse/" + jiraTicketKey
	}

	// Step 9: Fire-and-forget POST to MCP Dashboard (if configured).
	go postToDashboard(req, claudeResult, buildCtx, teamAssignment, githubIssueURL, jiraTicketKey, jiraTicketUrl, time.Since(start).Milliseconds())

	// Step 10: Build and return the result.
	result := &models.AnalysisResult{
		Status:           "completed",
		Category:         claudeResult.Category,
		RootCauseSummary: claudeResult.RootCauseSummary,
		RootCauseDetails: claudeResult.RootCauseDetails,
		ResponsibleTeam:  teamAssignment.Name,
		TeamEmail:        teamAssignment.Email,
		HtmlReport:       htmlReport,
		GithubIssueUrl:   githubIssueURL,
		JiraTicketKey:    jiraTicketKey,
		JiraTicketUrl:    jiraTicketUrl,
		Evidence:         claudeResult.Evidence,
		NextSteps:        claudeResult.NextSteps,
		AnalysisTimeMs:   time.Since(start).Milliseconds(),
		ClaudeAnalysis:   *claudeResult,
	}

	return result, nil
}

// runAgentsParallel selects and runs MCP agents based on software category selection.
func runAgentsParallel(ctx context.Context, req *models.AnalysisRequest, buildCtx *models.BuildContext) *models.McpResults {
	results := &models.McpResults{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	run := func(name string, fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("starting %s agent", name)
			fn()
		}()
	}

	// SCM agent selection based on categories.repoSoftware
	switch strings.ToLower(req.Categories.RepoSoftware) {
	case "bitbucket":
		run("bitbucket", func() {
			result, err := agents.NewBitBucketAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: bitbucket agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.BitBucketResult = result
				mu.Unlock()
			}
		})
	case "github", "":
		// Default: run GitHub agent (we're inside GitHub Actions)
		run("github", func() {
			result, err := agents.NewGithubAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: github agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.GithubResult = result
				mu.Unlock()
			}
		})
	}

	// Cluster agent selection based on categories.clusterType
	switch strings.ToLower(req.Categories.ClusterType) {
	case "kubernetes":
		run("kubernetes", func() {
			result, err := agents.NewKubernetesAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: kubernetes agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.KubernetesResult = result
				mu.Unlock()
			}
		})
	case "docker":
		run("docker", func() {
			result, err := agents.NewDockerAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: docker agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.DockerResult = result
				mu.Unlock()
			}
		})
	default:
		// Auto: run whichever has config
		if req.Kubernetes.ApiUrl != "" {
			run("kubernetes", func() {
				result, err := agents.NewKubernetesAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: kubernetes agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.KubernetesResult = result
					mu.Unlock()
				}
			})
		}
		if req.Docker.Host != "" {
			run("docker", func() {
				result, err := agents.NewDockerAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: docker agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.DockerResult = result
					mu.Unlock()
				}
			})
		}
	}

	// Artifact manager selection based on categories.artifactManager
	switch strings.ToLower(req.Categories.ArtifactManager) {
	case "jfrog":
		run("jfrog", func() {
			result, err := agents.NewJFrogAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: jfrog agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.JFrogResult = result
				mu.Unlock()
			}
		})
	case "nexus":
		run("nexus", func() {
			result, err := agents.NewNexusAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: nexus agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.NexusResult = result
				mu.Unlock()
			}
		})
	default:
		if req.JFrog.Url != "" {
			run("jfrog", func() {
				result, err := agents.NewJFrogAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: jfrog agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.JFrogResult = result
					mu.Unlock()
				}
			})
		}
		if req.Nexus.Url != "" {
			run("nexus", func() {
				result, err := agents.NewNexusAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: nexus agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.NexusResult = result
					mu.Unlock()
				}
			})
		}
	}

	wg.Wait()
	return results
}

// fetchWorkflowLogs downloads and extracts workflow run logs from the GitHub API.
func fetchWorkflowLogs(ctx context.Context, req *models.AnalysisRequest) (string, error) {
	apiURL := req.ApiUrl
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}

	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/logs",
		apiURL, req.Owner, req.Repo, req.RunID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.GithubToken)
	httpReq.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}

	var combined strings.Builder
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("read zip entry %s: %w", f.Name, err)
		}

		combined.WriteString(fmt.Sprintf("=== %s ===\n", f.Name))
		combined.Write(content)
		combined.WriteString("\n")
	}

	return combined.String(), nil
}

// postToDashboard sends analysis results to the MCP Dashboard (if MCP_DASHBOARD_URL is set).
func postToDashboard(req *models.AnalysisRequest, claudeResult *models.ClaudeAnalysis, buildCtx *models.BuildContext, teamMgr *models.TeamManager, githubIssueURL, jiraTicketKey, jiraTicketUrl string, analysisTimeMs int64) {
	dashURL := os.Getenv("MCP_DASHBOARD_URL")
	if dashURL == "" {
		return
	}

	payload := map[string]interface{}{
		"analysisId":       req.AnalysisID,
		"owner":            req.Owner,
		"repo":             req.Repo,
		"workflow":         req.Workflow,
		"runId":            req.RunID,
		"runNumber":        req.RunNumber,
		"actor":            req.Actor,
		"sha":              req.SHA,
		"ref":              req.Ref,
		"failedStep":       buildCtx.FailedStep,
		"failedJob":        buildCtx.FailedJob,
		"status":           "completed",
		"category":         claudeResult.Category,
		"rootCauseSummary": claudeResult.RootCauseSummary,
		"rootCauseDetails": claudeResult.RootCauseDetails,
		"responsibleTeam":  teamMgr.Name,
		"teamEmail":        teamMgr.Email,
		"confidence":       claudeResult.Confidence,
		"evidence":         claudeResult.Evidence,
		"nextSteps":        claudeResult.NextSteps,
		"errorMessages":    buildCtx.ErrorMessages,
		"analysisTimeMs":   analysisTimeMs,
		"jiraTicketKey":    jiraTicketKey,
		"jiraTicketUrl":    jiraTicketUrl,
		"githubIssueUrl":   githubIssueURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("warning: dashboard payload marshal: %v", err)
		return
	}

	url := strings.TrimRight(dashURL, "/") + "/api/ingest/github"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("warning: dashboard POST failed: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("dashboard: posted to %s (status %d)", url, resp.StatusCode)
}
