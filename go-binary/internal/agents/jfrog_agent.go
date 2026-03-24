package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// dependencyPatterns matches common dependency-related error phrases.
var dependencyPatterns = regexp.MustCompile(
	`(?i)(?:could not resolve|failed to download|artifact not found|dependency .+ not found|unable to resolve|` +
		`cannot find|no matching version|package .+ not available)`)

// JFrogAgent checks JFrog Artifactory for artifact availability and
// dependency-related build failure evidence.
type JFrogAgent struct {
	url      string
	username string
	apiKey   string
}

// NewJFrogAgent creates a JFrogAgent from the analysis request config.
func NewJFrogAgent(req *models.AnalysisRequest) *JFrogAgent {
	return &JFrogAgent{
		url:      strings.TrimRight(req.JFrog.Url, "/"),
		username: req.JFrog.Username,
		apiKey:   req.JFrog.ApiKey,
	}
}

// Analyze checks JFrog Artifactory system health and scans build error
// messages for dependency-related patterns. Returns nil if JFrog is not configured.
func (j *JFrogAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.JFrogAgentResult, error) {
	if j.url == "" || j.apiKey == "" {
		log.Println("[jfrog] skipping: jfrog config is empty")
		return nil, nil
	}

	result := &models.JFrogAgentResult{
		ArtifactsAvailable: true,
	}

	// Check Artifactory system health / ping.
	pingURL := fmt.Sprintf("%s/api/system/ping", j.url)
	pingData, err := j.get(ctx, pingURL)
	if err != nil {
		result.RepositoryStatus = fmt.Sprintf("unhealthy: %v", err)
		result.ArtifactsAvailable = false
	} else {
		body := strings.TrimSpace(string(pingData))
		if strings.EqualFold(body, "OK") {
			result.RepositoryStatus = "healthy"
		} else {
			result.RepositoryStatus = fmt.Sprintf("unexpected response: %s", body)
		}
	}

	// Check storage info for potential disk/quota issues.
	storageURL := fmt.Sprintf("%s/api/storageinfo", j.url)
	storageData, err := j.get(ctx, storageURL)
	if err != nil {
		log.Printf("[jfrog] warning: failed to fetch storage info: %v", err)
	} else {
		j.parseStorageInfo(storageData, result)
	}

	// Scan build error messages for dependency-related patterns.
	j.scanForMissingArtifacts(buildCtx, result)

	return result, nil
}

// get performs an authenticated GET against the JFrog API.
// Prefers username+apiKey basic auth; falls back to X-JFrog-Art-Api header.
func (j *JFrogAgent) get(ctx context.Context, url string) ([]byte, error) {
	if j.username != "" && j.apiKey != "" {
		return doRequest(ctx, url, "Authorization", basicAuthValue(j.username, j.apiKey))
	}
	return doRequest(ctx, url, "X-JFrog-Art-Api", j.apiKey)
}

// parseStorageInfo checks the storage summary for quota warnings.
func (j *JFrogAgent) parseStorageInfo(data []byte, result *models.JFrogAgentResult) {
	var storage struct {
		BinariesSummary struct {
			UsedSpace  string `json:"usedSpace"`
			FreeSpace  string `json:"freeSpace"`
			Optimization string `json:"optimization"`
		} `json:"binariesSummary"`
	}
	if err := json.Unmarshal(data, &storage); err != nil {
		log.Printf("[jfrog] warning: failed to parse storage info: %v", err)
		return
	}
	if storage.BinariesSummary.FreeSpace != "" {
		log.Printf("[jfrog] storage: used=%s free=%s", storage.BinariesSummary.UsedSpace, storage.BinariesSummary.FreeSpace)
	}
}

// scanForMissingArtifacts examines the build context error messages for
// dependency resolution failures, which may indicate missing artifacts.
func (j *JFrogAgent) scanForMissingArtifacts(buildCtx *models.BuildContext, result *models.JFrogAgentResult) {
	for _, msg := range buildCtx.ErrorMessages {
		if dependencyPatterns.MatchString(msg) {
			result.MissingArtifacts = appendUniqueSafe(result.MissingArtifacts, msg)
			result.ArtifactsAvailable = false
		}
	}
	// Also scan the raw console log for dependency errors.
	if buildCtx.ConsoleLog != "" {
		for _, line := range strings.Split(buildCtx.ConsoleLog, "\n") {
			line = strings.TrimSpace(line)
			if dependencyPatterns.MatchString(line) && !containsSafe(result.MissingArtifacts, line) {
				result.MissingArtifacts = append(result.MissingArtifacts, line)
				result.ArtifactsAvailable = false
			}
		}
	}
}

func appendUniqueSafe(slice []string, s string) []string {
	if !containsSafe(slice, s) {
		return append(slice, s)
	}
	return slice
}

func containsSafe(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
