package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// GithubAgent gathers contextual data from GitHub APIs for build failure analysis.
type GithubAgent struct {
	req    *models.AnalysisRequest
	apiUrl string
	token  string
	owner  string
	repo   string
}

// NewGithubAgent creates a GithubAgent configured from the analysis request.
func NewGithubAgent(req *models.AnalysisRequest) *GithubAgent {
	apiUrl := req.ApiUrl
	if apiUrl == "" {
		apiUrl = "https://api.github.com"
	}
	apiUrl = strings.TrimRight(apiUrl, "/")

	return &GithubAgent{
		req:    req,
		apiUrl: apiUrl,
		token:  req.GithubToken,
		owner:  req.Owner,
		repo:   req.Repo,
	}
}

// Analyze fetches workflow run details, jobs, commits, CODEOWNERS, and optional
// PR information from the GitHub API.
func (g *GithubAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.GithubAgentResult, error) {
	result := &models.GithubAgentResult{}

	// Fetch workflow run.
	runURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d", g.apiUrl, g.owner, g.repo, g.req.RunID)
	runData, err := g.get(ctx, runURL)
	if err != nil {
		log.Printf("[github] warning: failed to fetch workflow run: %v", err)
	} else {
		result.WorkflowRun = string(runData)
	}

	// Fetch jobs for the workflow run.
	jobsURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/jobs", g.apiUrl, g.owner, g.repo, g.req.RunID)
	jobsData, err := g.get(ctx, jobsURL)
	if err != nil {
		log.Printf("[github] warning: failed to fetch jobs: %v", err)
	} else {
		result.Jobs = parseJobs(jobsData)
	}

	// Fetch recent commits on the ref.
	ref := g.req.Ref
	if ref == "" {
		ref = "main"
	}
	commitsURL := fmt.Sprintf("%s/repos/%s/%s/commits?sha=%s&per_page=20", g.apiUrl, g.owner, g.repo, ref)
	commitsData, err := g.get(ctx, commitsURL)
	if err != nil {
		log.Printf("[github] warning: failed to fetch commits: %v", err)
	} else {
		result.RecentCommits = parseCommits(commitsData)
	}

	// Fetch CODEOWNERS (try root first, then .github/).
	result.Codeowners = g.fetchCodeowners(ctx)

	// If this is a pull request event, fetch PR details and changed files.
	if g.req.PullRequestNumber > 0 {
		g.fetchPRDetails(ctx, result)
	}

	return result, nil
}

// get performs an authenticated GET request against the GitHub API.
func (g *GithubAgent) get(ctx context.Context, url string) ([]byte, error) {
	return doRequest(ctx, url, "Authorization", "Bearer "+g.token)
}

// fetchCodeowners tries to retrieve the CODEOWNERS file from known locations.
func (g *GithubAgent) fetchCodeowners(ctx context.Context) string {
	paths := []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"}
	for _, p := range paths {
		url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", g.apiUrl, g.owner, g.repo, p)
		data, err := g.get(ctx, url)
		if err != nil {
			continue
		}
		// The contents API returns JSON with a "content" field (base64-encoded).
		var contents struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if json.Unmarshal(data, &contents) == nil && contents.Content != "" {
			return contents.Content
		}
	}
	return ""
}

// fetchPRDetails retrieves the pull request description and list of changed files.
func (g *GithubAgent) fetchPRDetails(ctx context.Context, result *models.GithubAgentResult) {
	prURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", g.apiUrl, g.owner, g.repo, g.req.PullRequestNumber)
	prData, err := g.get(ctx, prURL)
	if err != nil {
		log.Printf("[github] warning: failed to fetch PR #%d: %v", g.req.PullRequestNumber, err)
		return
	}
	var pr struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if json.Unmarshal(prData, &pr) == nil {
		result.PrTitle = pr.Title
		result.PrBody = pr.Body
	}

	// Fetch changed files.
	filesURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files", g.apiUrl, g.owner, g.repo, g.req.PullRequestNumber)
	filesData, err := g.get(ctx, filesURL)
	if err != nil {
		log.Printf("[github] warning: failed to fetch PR files: %v", err)
		return
	}
	var files []struct {
		Filename string `json:"filename"`
	}
	if json.Unmarshal(filesData, &files) == nil {
		for _, f := range files {
			result.ChangedFiles = append(result.ChangedFiles, f.Filename)
		}
	}
}

// parseJobs extracts job and step information from the GitHub jobs API response.
func parseJobs(data []byte) []models.JobInfo {
	var resp struct {
		Jobs []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			Steps      []struct {
				Name       string `json:"name"`
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
			} `json:"steps"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		log.Printf("[github] warning: failed to parse jobs: %v", err)
		return nil
	}

	jobs := make([]models.JobInfo, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		job := models.JobInfo{
			Name:       j.Name,
			Status:     j.Status,
			Conclusion: j.Conclusion,
		}
		for _, s := range j.Steps {
			job.Steps = append(job.Steps, models.StepInfo{
				Name:       s.Name,
				Status:     s.Status,
				Conclusion: s.Conclusion,
			})
		}
		jobs = append(jobs, job)
	}
	return jobs
}

// parseCommits extracts commit information from the GitHub commits API response.
func parseCommits(data []byte) []models.CommitInfo {
	var raw []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("[github] warning: failed to parse commits: %v", err)
		return nil
	}

	commits := make([]models.CommitInfo, 0, len(raw))
	for _, c := range raw {
		commits = append(commits, models.CommitInfo{
			SHA:     c.SHA,
			Author:  c.Commit.Author.Name,
			Message: c.Commit.Message,
			Date:    c.Commit.Author.Date,
		})
	}
	return commits
}
