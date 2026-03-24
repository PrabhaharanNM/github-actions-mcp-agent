package team

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// jobRepoPattern matches job names like "Build - orders", "payments Build",
// "orders - Build", "Deploy - myservice".
var jobRepoPattern = regexp.MustCompile(`(?i)(?:Build\s*[-:]\s*(\S+))|(?:(\S+)\s*[-:]\s*Build)|(?:(\S+)\s+Build)|(?:Deploy\s*[-:]\s*(\S+))`)

// Assign determines the responsible team manager based on the correlation
// analysis, team mappings, and build context.
func Assign(req *models.AnalysisRequest, buildCtx *models.BuildContext, corr *models.Correlation) *models.TeamManager {
	teamMappings := parseTeamMappings(req.TeamMappings)
	devopsManager := parseDevopsManager(req.DevopsManager)

	// If the issue is infrastructure, assign to DevOps.
	if corr.IsInfrastructure {
		return devopsManager
	}

	// If a responsible repository was identified, look it up in team mappings.
	if corr.ResponsibleRepository != "" {
		if mgr, ok := lookupManager(teamMappings, corr.ResponsibleRepository); ok {
			return mgr
		}
	}

	// Try to extract a repository name from the failed job.
	if buildCtx.FailedJob != "" {
		repo := extractRepoFromJob(buildCtx.FailedJob)
		if repo != "" {
			if mgr, ok := lookupManager(teamMappings, repo); ok {
				return mgr
			}
		}
	}

	// Try to extract from the failed step.
	if buildCtx.FailedStep != "" {
		repo := extractRepoFromJob(buildCtx.FailedStep)
		if repo != "" {
			if mgr, ok := lookupManager(teamMappings, repo); ok {
				return mgr
			}
		}
	}

	// Fallback to DevOps manager.
	return devopsManager
}

// parseTeamMappings parses the JSON team mappings string into a map.
func parseTeamMappings(raw string) map[string]models.TeamManager {
	if raw == "" {
		return nil
	}
	var mappings map[string]models.TeamManager
	if err := json.Unmarshal([]byte(raw), &mappings); err != nil {
		log.Printf("warning: failed to parse team mappings JSON: %v", err)
		return nil
	}
	return mappings
}

// parseDevopsManager parses the JSON devops manager string into a TeamManager.
func parseDevopsManager(raw string) *models.TeamManager {
	defaultManager := &models.TeamManager{
		Name:  "DevOps Team",
		Email: "",
	}

	if raw == "" {
		return defaultManager
	}

	var mgr models.TeamManager
	if err := json.Unmarshal([]byte(raw), &mgr); err != nil {
		log.Printf("warning: failed to parse devops manager JSON: %v", err)
		return defaultManager
	}
	return &mgr
}

// lookupManager searches the team mappings for a repository name (case-insensitive).
func lookupManager(mappings map[string]models.TeamManager, repo string) (*models.TeamManager, bool) {
	if mappings == nil {
		return nil, false
	}

	repoLower := strings.ToLower(strings.TrimSpace(repo))

	// Exact match first.
	for key, mgr := range mappings {
		if strings.ToLower(key) == repoLower {
			m := mgr
			return &m, true
		}
	}

	// Partial match: check if repo is contained in a key or vice versa.
	for key, mgr := range mappings {
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, repoLower) || strings.Contains(repoLower, keyLower) {
			m := mgr
			return &m, true
		}
	}

	return nil, false
}

// extractRepoFromJob uses regex to extract a repository identifier from
// job names like "Build - orders", "payments Build", "Deploy - myservice".
func extractRepoFromJob(job string) string {
	matches := jobRepoPattern.FindStringSubmatch(job)
	if matches == nil {
		return ""
	}

	// Return the first non-empty capture group.
	for i := 1; i < len(matches); i++ {
		if matches[i] != "" {
			return strings.TrimSpace(matches[i])
		}
	}

	return ""
}
