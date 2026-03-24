package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// NexusAgent checks Sonatype Nexus for artifact availability and
// repository health related to build failures.
type NexusAgent struct {
	url      string
	username string
	password string
}

// NewNexusAgent creates a NexusAgent from the analysis request config.
func NewNexusAgent(req *models.AnalysisRequest) *NexusAgent {
	return &NexusAgent{
		url:      strings.TrimRight(req.Nexus.Url, "/"),
		username: req.Nexus.Username,
		password: req.Nexus.Password,
	}
}

// Analyze queries Sonatype Nexus for artifact search results and repository
// status. Returns nil if Nexus is not configured.
func (n *NexusAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.NexusAgentResult, error) {
	if n.url == "" {
		log.Println("[nexus] skipping: nexus config is empty")
		return nil, nil
	}

	result := &models.NexusAgentResult{
		ArtifactsAvailable: true,
	}

	// Check Nexus repository status.
	statusURL := fmt.Sprintf("%s/service/rest/v1/status", n.url)
	_, err := n.get(ctx, statusURL)
	if err != nil {
		result.RepositoryStatus = fmt.Sprintf("unhealthy: %v", err)
		result.ArtifactsAvailable = false
	} else {
		result.RepositoryStatus = "healthy"
	}

	// Derive artifact name from the build context.
	artifactName := n.deriveArtifactName(buildCtx)
	if artifactName != "" {
		n.searchArtifact(ctx, artifactName, result)
	}

	// Scan build error messages for dependency-related patterns.
	n.scanForMissingArtifacts(buildCtx, result)

	return result, nil
}

// get performs an authenticated GET against the Nexus REST API using basic auth.
func (n *NexusAgent) get(ctx context.Context, url string) ([]byte, error) {
	if n.username != "" && n.password != "" {
		return doRequest(ctx, url, "Authorization", basicAuthValue(n.username, n.password))
	}
	return doRequest(ctx, url, "", "")
}

// deriveArtifactName attempts to derive an artifact name from the build context.
// It uses the repository name or workflow name as the search term.
func (n *NexusAgent) deriveArtifactName(buildCtx *models.BuildContext) string {
	if buildCtx.Repo != "" {
		// Use the repo name (without owner prefix) as the artifact name.
		parts := strings.Split(buildCtx.Repo, "/")
		if len(parts) > 1 {
			return parts[len(parts)-1]
		}
		return buildCtx.Repo
	}
	if buildCtx.Workflow != "" {
		return buildCtx.Workflow
	}
	return ""
}

// searchArtifact searches Nexus for a given artifact name and records any
// missing artifacts in the result.
func (n *NexusAgent) searchArtifact(ctx context.Context, artifactName string, result *models.NexusAgentResult) {
	searchURL := fmt.Sprintf("%s/service/rest/v1/search?name=%s", n.url, artifactName)
	data, err := n.get(ctx, searchURL)
	if err != nil {
		log.Printf("[nexus] warning: failed to search for artifact %s: %v", artifactName, err)
		return
	}

	var searchResp struct {
		Items []struct {
			ID         string `json:"id"`
			Repository string `json:"repository"`
			Name       string `json:"name"`
			Version    string `json:"version"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &searchResp); err != nil {
		log.Printf("[nexus] warning: failed to parse search response: %v", err)
		return
	}

	if len(searchResp.Items) == 0 {
		result.MissingArtifacts = append(result.MissingArtifacts,
			fmt.Sprintf("artifact %q not found in Nexus", artifactName))
		result.ArtifactsAvailable = false
	} else {
		log.Printf("[nexus] found %d artifacts matching %q", len(searchResp.Items), artifactName)
	}
}

// scanForMissingArtifacts examines the build context error messages for
// dependency resolution failures, which may indicate missing artifacts.
func (n *NexusAgent) scanForMissingArtifacts(buildCtx *models.BuildContext, result *models.NexusAgentResult) {
	for _, msg := range buildCtx.ErrorMessages {
		if dependencyPatterns.MatchString(msg) {
			result.MissingArtifacts = appendUniqueSafe(result.MissingArtifacts, msg)
			result.ArtifactsAvailable = false
		}
	}
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
