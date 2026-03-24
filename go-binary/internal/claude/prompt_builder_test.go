package claude

import (
	"strings"
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

func TestBuildUserPrompt_IncludesWorkflowInfo(t *testing.T) {
	buildCtx := &models.BuildContext{
		Owner:    "myorg",
		Repo:     "myrepo",
		Workflow: "CI",
		RunID:    12345,
		SHA:      "abc123def456",
	}
	prompt := BuildUserPrompt(buildCtx, nil, nil)
	if !strings.Contains(prompt, "myorg") {
		t.Error("expected prompt to contain owner")
	}
	if !strings.Contains(prompt, "myrepo") {
		t.Error("expected prompt to contain repo")
	}
	if !strings.Contains(prompt, "CI") {
		t.Error("expected prompt to contain workflow name")
	}
}

func TestBuildUserPrompt_IncludesGithubData(t *testing.T) {
	buildCtx := &models.BuildContext{}
	mcpResults := &models.McpResults{
		GithubResult: &models.GithubAgentResult{
			PrTitle: "Fix the bug",
			RecentCommits: []models.CommitInfo{
				{SHA: "abc123", Author: "dev", Message: "commit msg"},
			},
		},
	}
	prompt := BuildUserPrompt(buildCtx, mcpResults, nil)
	if !strings.Contains(prompt, "Fix the bug") {
		t.Error("expected prompt to contain PR title")
	}
	if !strings.Contains(prompt, "commit msg") {
		t.Error("expected prompt to contain commit message")
	}
}

func TestBuildUserPrompt_NilInputsDoNotPanic(t *testing.T) {
	// Should not panic with all nil inputs.
	prompt := BuildUserPrompt(nil, nil, nil)
	if prompt == "" {
		t.Error("expected non-empty prompt even with nil inputs")
	}
}

// --- Section-specific tests ---

func TestBuildUserPrompt_BitBucketDataSection(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		BitBucketResult: &models.BitBucketAgentResult{
			CodeOwners: "* @platform-team",
			RecentCommits: []models.CommitInfo{
				{SHA: "bb12345678abc", Message: "update config", Author: "bb-dev", Date: "2024-03-01"},
			},
			ChangedFiles: []string{"config/app.yml", "src/main.go"},
		},
	}

	prompt := BuildUserPrompt(buildCtx, results, nil)

	if !strings.Contains(prompt, "BITBUCKET DATA") {
		t.Error("prompt should contain BITBUCKET DATA section")
	}
	if !strings.Contains(prompt, "update config") {
		t.Error("prompt should contain commit message")
	}
	if !strings.Contains(prompt, "config/app.yml") {
		t.Error("prompt should contain changed files")
	}
	if !strings.Contains(prompt, "platform-team") {
		t.Error("prompt should contain CODEOWNERS")
	}
	// Verify hash shortened to 8 chars
	if !strings.Contains(prompt, "bb123456") {
		t.Error("prompt should contain shortened hash")
	}
}

func TestBuildUserPrompt_DockerDataSection(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			ContainerStatuses: []models.ContainerStatus{
				{Name: "web", Image: "nginx:1.25", State: "running", Status: "Up 1h", ExitCode: 0},
				{Name: "api", Image: "myapp:v2", State: "exited", Status: "Exited (1)", ExitCode: 1},
			},
			FailedContainers: []string{"api exited(1)"},
			OOMKilled:        []string{"worker"},
			ImageIssues:      []string{"myapp:v3 not found"},
			DiskUsage:        "60GB / 100GB",
		},
	}

	prompt := BuildUserPrompt(buildCtx, results, nil)

	if !strings.Contains(prompt, "DOCKER DATA") {
		t.Error("prompt should contain DOCKER DATA section")
	}
	if !strings.Contains(prompt, "web") {
		t.Error("prompt should contain container name")
	}
	if !strings.Contains(prompt, "nginx:1.25") {
		t.Error("prompt should contain image name")
	}
	if !strings.Contains(prompt, "Failed Containers") {
		t.Error("prompt should contain failed containers")
	}
	if !strings.Contains(prompt, "OOM Killed") {
		t.Error("prompt should contain OOM killed")
	}
	if !strings.Contains(prompt, "Image Issues") {
		t.Error("prompt should contain image issues")
	}
	if !strings.Contains(prompt, "60GB") {
		t.Error("prompt should contain disk usage")
	}
}

func TestBuildUserPrompt_NexusDataSection(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: false,
			RepositoryStatus:   "healthy",
			MissingArtifacts:   []string{"org.example:core:2.0", "org.example:utils:2.0"},
		},
	}

	prompt := BuildUserPrompt(buildCtx, results, nil)

	if !strings.Contains(prompt, "NEXUS DATA") {
		t.Error("prompt should contain NEXUS DATA section")
	}
	if !strings.Contains(prompt, "Artifacts Available: false") {
		t.Error("prompt should show artifacts unavailable")
	}
	if !strings.Contains(prompt, "org.example:core:2.0") {
		t.Error("prompt should contain missing artifact")
	}
}

func TestBuildUserPrompt_DockerEmpty(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{},
	}

	prompt := BuildUserPrompt(buildCtx, results, nil)
	if !strings.Contains(prompt, "DOCKER DATA") {
		t.Error("prompt should contain DOCKER DATA section")
	}
	if strings.Contains(prompt, "Failed Containers") {
		t.Error("should not show Failed Containers when empty")
	}
}

func TestBuildUserPrompt_ConsoleLogTruncation(t *testing.T) {
	var lines []string
	for i := 0; i < 300; i++ {
		lines = append(lines, "log line")
	}
	buildCtx := &models.BuildContext{
		ConsoleLog: strings.Join(lines, "\n"),
	}

	prompt := BuildUserPrompt(buildCtx, nil, nil)
	if !strings.Contains(prompt, "truncated") {
		t.Error("prompt should indicate truncation for >200 lines")
	}
}

func TestBuildUserPrompt_ErrorMessageLimit(t *testing.T) {
	var errors []string
	for i := 0; i < 30; i++ {
		errors = append(errors, "error message")
	}
	buildCtx := &models.BuildContext{
		ErrorMessages: errors,
	}

	prompt := BuildUserPrompt(buildCtx, nil, nil)
	if !strings.Contains(prompt, "and 10 more") {
		t.Error("prompt should indicate truncated error messages")
	}
}

func TestBuildUserPrompt_CorrelationSection(t *testing.T) {
	buildCtx := &models.BuildContext{}
	corr := &models.Correlation{
		RootCauseType:         "DependencyIssue",
		IsInfrastructure:      false,
		ResponsibleRepository: "myapp",
		ResponsibleTeam:       "Platform",
		Evidence:              []string{"Missing Nexus artifact: org.example:core:2.0"},
	}

	prompt := BuildUserPrompt(buildCtx, nil, corr)

	if !strings.Contains(prompt, "DependencyIssue") {
		t.Error("prompt should contain root cause type")
	}
	if !strings.Contains(prompt, "Platform") {
		t.Error("prompt should contain responsible team")
	}
	if !strings.Contains(prompt, "Missing Nexus artifact") {
		t.Error("prompt should contain evidence")
	}
}

func TestBuildUserPrompt_AllSectionsPresent(t *testing.T) {
	buildCtx := &models.BuildContext{
		Owner:    "org",
		Repo:     "repo",
		Workflow: "CI",
	}
	results := &models.McpResults{
		GithubResult:     &models.GithubAgentResult{},
		BitBucketResult:  &models.BitBucketAgentResult{},
		KubernetesResult: &models.KubernetesAgentResult{},
		DockerResult:     &models.DockerAgentResult{},
		JFrogResult:      &models.JFrogAgentResult{},
		NexusResult:      &models.NexusAgentResult{},
	}
	corr := &models.Correlation{RootCauseType: "Unknown"}

	prompt := BuildUserPrompt(buildCtx, results, corr)

	requiredSections := []string{
		"WORKFLOW INFORMATION",
		"FAILED JOB / STEP",
		"ERROR MESSAGES",
		"CONSOLE LOG",
		"GITHUB DATA",
		"BITBUCKET DATA",
		"KUBERNETES DATA",
		"DOCKER DATA",
		"JFROG DATA",
		"NEXUS DATA",
		"CROSS-CORRELATION ANALYSIS",
	}
	for _, section := range requiredSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt missing required section: %s", section)
		}
	}
}

func TestBuildUserPrompt_PullRequestInfo(t *testing.T) {
	buildCtx := &models.BuildContext{
		PullRequestNumber: 42,
	}
	prompt := BuildUserPrompt(buildCtx, nil, nil)
	if !strings.Contains(prompt, "#42") {
		t.Error("prompt should contain PR number")
	}
}

func TestShortHash(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc123def456789", "abc123de"},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"", ""},
	}
	for _, tt := range tests {
		got := shortHash(tt.input)
		if got != tt.want {
			t.Errorf("shortHash(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
