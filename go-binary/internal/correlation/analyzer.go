package correlation

import (
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// Analyze performs deterministic cross-correlation across all MCP agent results
// to identify the most likely root cause category and responsible team/repository.
func Analyze(buildCtx *models.BuildContext, results *models.McpResults) *models.Correlation {
	// Priority 1: Suspected repository with supporting evidence.
	if corr := checkSuspectedRepository(buildCtx, results); corr != nil {
		return corr
	}

	// Priority 2: Failed job/step contains "build" — likely a code change issue.
	if corr := checkBuildJobFailure(buildCtx); corr != nil {
		return corr
	}

	// Priority 3: Kubernetes OOM kills or node pressure.
	if corr := checkKubernetesIssues(results); corr != nil {
		return corr
	}

	// Priority 4: Docker container failures (OOM, non-zero exit).
	if corr := checkDockerIssues(results); corr != nil {
		return corr
	}

	// Priority 5: JFrog missing artifacts.
	if corr := checkJFrogIssues(results); corr != nil {
		return corr
	}

	// Priority 6: Nexus missing artifacts.
	if corr := checkNexusIssues(results); corr != nil {
		return corr
	}

	// Priority 7: Default — treat as infrastructure issue.
	return &models.Correlation{
		RootCauseType:    "Infrastructure",
		IsInfrastructure: true,
		ResponsibleTeam:  "DevOps",
		Evidence:         []string{"No specific root cause identified; defaulting to infrastructure investigation"},
	}
}

// checkSuspectedRepository checks whether the suspected repository is supported
// by error messages referencing code files or recent commits from GitHub.
func checkSuspectedRepository(buildCtx *models.BuildContext, results *models.McpResults) *models.Correlation {
	if buildCtx.SuspectedRepository == "" {
		return nil
	}

	var evidence []string
	found := false

	// Check if error messages reference code files (common extensions).
	codeExtensions := []string{".java", ".go", ".py", ".js", ".ts", ".cs", ".cpp", ".c", ".rb", ".scala", ".kt"}
	for _, msg := range buildCtx.ErrorMessages {
		for _, ext := range codeExtensions {
			if strings.Contains(msg, ext) {
				evidence = append(evidence, "Error message references code file: "+truncate(msg, 120))
				found = true
				break
			}
		}
	}

	// Check if recent commits exist from GitHub or BitBucket data.
	if results != nil && results.GithubResult != nil && len(results.GithubResult.RecentCommits) > 0 {
		for _, commit := range results.GithubResult.RecentCommits {
			evidence = append(evidence, "Recent commit by "+commit.Author+": "+truncate(commit.Message, 80))
			found = true
		}
	}
	if results != nil && results.BitBucketResult != nil && len(results.BitBucketResult.RecentCommits) > 0 {
		for _, commit := range results.BitBucketResult.RecentCommits {
			evidence = append(evidence, "Recent commit by "+commit.Author+": "+truncate(commit.Message, 80))
			found = true
		}
	}

	if !found {
		return nil
	}

	return &models.Correlation{
		RootCauseType:         "CodeChange",
		IsInfrastructure:      false,
		ResponsibleRepository: buildCtx.SuspectedRepository,
		Evidence:              evidence,
	}
}

// checkBuildJobFailure checks whether the failed job or step name contains "build",
// indicating a compilation or code-level failure.
func checkBuildJobFailure(buildCtx *models.BuildContext) *models.Correlation {
	failedName := buildCtx.FailedJob
	if failedName == "" {
		failedName = buildCtx.FailedStep
	}
	if failedName == "" {
		return nil
	}

	if !strings.Contains(strings.ToLower(failedName), "build") {
		return nil
	}

	evidence := []string{"Failed job/step contains 'build': " + failedName}

	repo := extractRepoFromJobName(failedName)
	if repo == "" {
		repo = buildCtx.Repo
	}

	return &models.Correlation{
		RootCauseType:         "CodeChange",
		IsInfrastructure:      false,
		ResponsibleRepository: repo,
		Evidence:              evidence,
	}
}

// checkKubernetesIssues looks for OOM kills or node pressure in K8s results.
func checkKubernetesIssues(results *models.McpResults) *models.Correlation {
	if results == nil || results.KubernetesResult == nil {
		return nil
	}

	k8s := results.KubernetesResult
	hasOOM := len(k8s.OOMKills) > 0
	hasPressure := k8s.NodePressure

	if !hasOOM && !hasPressure {
		return nil
	}

	var evidence []string
	if hasOOM {
		for _, oom := range k8s.OOMKills {
			evidence = append(evidence, "OOM kill detected: "+oom)
		}
	}
	if hasPressure {
		evidence = append(evidence, "Kubernetes node pressure detected")
	}

	return &models.Correlation{
		RootCauseType:    "Infrastructure",
		IsInfrastructure: true,
		ResponsibleTeam:  "DevOps",
		Evidence:         evidence,
	}
}

// checkJFrogIssues looks for missing artifacts in the JFrog results.
func checkJFrogIssues(results *models.McpResults) *models.Correlation {
	if results == nil || results.JFrogResult == nil {
		return nil
	}

	jfrog := results.JFrogResult
	if jfrog.ArtifactsAvailable || len(jfrog.MissingArtifacts) == 0 {
		return nil
	}

	var evidence []string
	for _, artifact := range jfrog.MissingArtifacts {
		evidence = append(evidence, "Missing artifact: "+artifact)
	}

	return &models.Correlation{
		RootCauseType:    "DependencyIssue",
		IsInfrastructure: false,
		Evidence:         evidence,
	}
}

// checkDockerIssues looks for OOM-killed or failed containers in Docker results.
func checkDockerIssues(results *models.McpResults) *models.Correlation {
	if results == nil || results.DockerResult == nil {
		return nil
	}

	docker := results.DockerResult
	hasOOM := len(docker.OOMKilled) > 0
	hasFailed := len(docker.FailedContainers) > 0

	if !hasOOM && !hasFailed {
		return nil
	}

	var evidence []string
	if hasOOM {
		for _, c := range docker.OOMKilled {
			evidence = append(evidence, "Docker container OOM killed: "+c)
		}
	}
	if hasFailed {
		for _, c := range docker.FailedContainers {
			evidence = append(evidence, "Docker container failed: "+c)
		}
	}

	return &models.Correlation{
		RootCauseType:    "Infrastructure",
		IsInfrastructure: true,
		ResponsibleTeam:  "DevOps",
		Evidence:         evidence,
	}
}

// checkNexusIssues looks for missing artifacts in the Nexus results.
func checkNexusIssues(results *models.McpResults) *models.Correlation {
	if results == nil || results.NexusResult == nil {
		return nil
	}

	nexus := results.NexusResult
	if nexus.ArtifactsAvailable || len(nexus.MissingArtifacts) == 0 {
		return nil
	}

	var evidence []string
	for _, artifact := range nexus.MissingArtifacts {
		evidence = append(evidence, "Missing Nexus artifact: "+artifact)
	}

	return &models.Correlation{
		RootCauseType:    "DependencyIssue",
		IsInfrastructure: false,
		Evidence:         evidence,
	}
}

// extractRepoFromJobName attempts to extract a repository identifier from
// job names like "Build - orders", "payments Build", "Deploy - myservice".
func extractRepoFromJobName(job string) string {
	job = strings.TrimSpace(job)

	// Pattern: "Build - XXX"
	if strings.HasPrefix(strings.ToLower(job), "build") {
		parts := strings.SplitN(job, "-", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	// Pattern: "XXX - Build" or "XXX Build"
	lower := strings.ToLower(job)
	if idx := strings.Index(lower, "build"); idx > 0 {
		prefix := strings.TrimSpace(job[:idx])
		prefix = strings.TrimRight(prefix, "- ")
		if prefix != "" {
			return prefix
		}
	}

	return ""
}

// truncate shortens a string to the given max length, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
