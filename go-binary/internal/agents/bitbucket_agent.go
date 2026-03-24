package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// BitBucketAgent gathers contextual data from BitBucket Server REST APIs
// for build failure analysis in environments using BitBucket as SCM.
type BitBucketAgent struct {
	url      string
	username string
	password string
}

// NewBitBucketAgent creates a BitBucketAgent from the analysis request config.
func NewBitBucketAgent(req *models.AnalysisRequest) *BitBucketAgent {
	return &BitBucketAgent{
		url:      strings.TrimRight(req.BitBucket.Url, "/"),
		username: req.BitBucket.Username,
		password: req.BitBucket.Password,
	}
}

// Analyze fetches recent commits, changed files, and code ownership information
// from BitBucket Server. Returns nil if BitBucket is not configured.
func (b *BitBucketAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.BitBucketAgentResult, error) {
	if b.url == "" || b.username == "" {
		log.Println("[bitbucket] skipping: bitbucket config is empty")
		return nil, nil
	}

	result := &models.BitBucketAgentResult{}

	// Derive project and repo from buildCtx.Repo (expected format: "project/repo").
	project, repo := b.splitRepo(buildCtx.Repo)
	if project == "" || repo == "" {
		log.Printf("[bitbucket] warning: could not derive project/repo from %q", buildCtx.Repo)
		return result, nil
	}

	// Fetch recent commits.
	commitsURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits?limit=10",
		b.url, project, repo)
	commitsData, err := b.get(ctx, commitsURL)
	if err != nil {
		log.Printf("[bitbucket] warning: failed to fetch commits: %v", err)
	} else {
		result.RecentCommits = b.parseCommits(commitsData)
	}

	// Fetch changed files for the current SHA.
	if buildCtx.SHA != "" {
		changesURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits/%s/changes",
			b.url, project, repo, buildCtx.SHA)
		changesData, err := b.get(ctx, changesURL)
		if err != nil {
			log.Printf("[bitbucket] warning: failed to fetch changes for %s: %v", buildCtx.SHA, err)
		} else {
			result.ChangedFiles = b.parseChangedFiles(changesData)
		}
	}

	// Attempt to fetch CODEOWNERS or similar ownership file.
	result.CodeOwners = b.fetchCodeOwners(ctx, project, repo)

	return result, nil
}

// get performs an authenticated GET against the BitBucket REST API using basic auth.
func (b *BitBucketAgent) get(ctx context.Context, url string) ([]byte, error) {
	return doRequest(ctx, url, "Authorization", basicAuthValue(b.username, b.password))
}

// splitRepo splits a "project/repo" string into project and repo components.
func (b *BitBucketAgent) splitRepo(repoPath string) (string, string) {
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// parseCommits extracts commit information from the BitBucket commits API response.
func (b *BitBucketAgent) parseCommits(data []byte) []models.CommitInfo {
	var resp struct {
		Values []struct {
			ID        string `json:"id"`
			Message   string `json:"message"`
			Author    struct {
				Name         string `json:"name"`
				EmailAddress string `json:"emailAddress"`
			} `json:"author"`
			AuthorTimestamp int64 `json:"authorTimestamp"`
		} `json:"values"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		log.Printf("[bitbucket] warning: failed to parse commits: %v", err)
		return nil
	}

	commits := make([]models.CommitInfo, 0, len(resp.Values))
	for _, c := range resp.Values {
		commits = append(commits, models.CommitInfo{
			SHA:     c.ID,
			Author:  c.Author.Name,
			Message: c.Message,
			Date:    fmt.Sprintf("%d", c.AuthorTimestamp),
		})
	}
	return commits
}

// parseChangedFiles extracts the list of changed file paths from the BitBucket
// commit changes API response.
func (b *BitBucketAgent) parseChangedFiles(data []byte) []string {
	var resp struct {
		Values []struct {
			Path struct {
				ToString string `json:"toString"`
			} `json:"path"`
		} `json:"values"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		log.Printf("[bitbucket] warning: failed to parse changed files: %v", err)
		return nil
	}

	files := make([]string, 0, len(resp.Values))
	for _, v := range resp.Values {
		if v.Path.ToString != "" {
			files = append(files, v.Path.ToString)
		}
	}
	return files
}

// fetchCodeOwners attempts to retrieve a CODEOWNERS file from known locations
// in the BitBucket repository.
func (b *BitBucketAgent) fetchCodeOwners(ctx context.Context, project, repo string) string {
	paths := []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"}
	for _, p := range paths {
		fileURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/browse/%s",
			b.url, project, repo, p)
		data, err := b.get(ctx, fileURL)
		if err != nil {
			continue
		}
		// BitBucket browse API returns JSON with "lines" containing the file content.
		var fileResp struct {
			Lines []struct {
				Text string `json:"text"`
			} `json:"lines"`
		}
		if json.Unmarshal(data, &fileResp) == nil && len(fileResp.Lines) > 0 {
			var lines []string
			for _, l := range fileResp.Lines {
				lines = append(lines, l.Text)
			}
			return strings.Join(lines, "\n")
		}
	}
	return ""
}
