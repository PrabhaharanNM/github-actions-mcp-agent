package correlation

import (
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// --- Priority 1: Suspected Repository ---

func TestAnalyze_CodeChangeWithGithubCommits(t *testing.T) {
	buildCtx := &models.BuildContext{
		SuspectedRepository: "myorg/myrepo",
		ErrorMessages:       []string{"error in src/main.go: undefined variable"},
	}
	results := &models.McpResults{
		GithubResult: &models.GithubAgentResult{
			RecentCommits: []models.CommitInfo{
				{SHA: "abc123", Author: "dev", Message: "fix handler"},
			},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
	if corr.ResponsibleRepository != "myorg/myrepo" {
		t.Errorf("expected myorg/myrepo, got %q", corr.ResponsibleRepository)
	}
}

func TestAnalyze_CodeChangeWithBitBucketCommits(t *testing.T) {
	buildCtx := &models.BuildContext{
		SuspectedRepository: "bb-repo",
		ErrorMessages:       []string{"cannot compile service.ts"},
	}
	results := &models.McpResults{
		BitBucketResult: &models.BitBucketAgentResult{
			RecentCommits: []models.CommitInfo{
				{SHA: "bb123", Author: "bb-dev", Message: "update service"},
			},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_CodeChangeWithBothSCMs(t *testing.T) {
	buildCtx := &models.BuildContext{
		SuspectedRepository: "cross-repo",
		ErrorMessages:       []string{"error in main.py"},
	}
	results := &models.McpResults{
		GithubResult: &models.GithubAgentResult{
			RecentCommits: []models.CommitInfo{
				{SHA: "gh1", Author: "gh-dev", Message: "gh commit"},
			},
		},
		BitBucketResult: &models.BitBucketAgentResult{
			RecentCommits: []models.CommitInfo{
				{SHA: "bb1", Author: "bb-dev", Message: "bb commit"},
			},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
	if len(corr.Evidence) < 3 {
		t.Errorf("expected at least 3 evidence items, got %d", len(corr.Evidence))
	}
}

func TestAnalyze_SuspectedRepoNoEvidence(t *testing.T) {
	buildCtx := &models.BuildContext{
		SuspectedRepository: "some-repo",
		ErrorMessages:       []string{"generic error"},
	}
	results := &models.McpResults{}
	corr := Analyze(buildCtx, results)
	// No code file extension match, no commits → falls through
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure fallthrough, got %q", corr.RootCauseType)
	}
}

// --- Priority 2: Build Job Failure ---

func TestAnalyze_BuildJobFailure(t *testing.T) {
	buildCtx := &models.BuildContext{
		FailedJob: "Build - orders",
	}
	results := &models.McpResults{}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_BuildStepFailure(t *testing.T) {
	buildCtx := &models.BuildContext{
		FailedStep: "Run build",
	}
	results := &models.McpResults{}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange from step, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_NonBuildJob(t *testing.T) {
	buildCtx := &models.BuildContext{
		FailedJob: "Deploy to Production",
	}
	results := &models.McpResults{}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
}

// --- Priority 3: Kubernetes Issues ---

func TestAnalyze_KubernetesOOMKill(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{
			OOMKills: []string{"pod/runner-1 container/main OOMKilled"},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
	if !corr.IsInfrastructure {
		t.Error("expected IsInfrastructure to be true")
	}
}

func TestAnalyze_KubernetesNodePressure(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{
			NodePressure: true,
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_KubernetesHealthy(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure default, got %q", corr.RootCauseType)
	}
}

// --- Priority 4: Docker Issues ---

func TestAnalyze_DockerOOMKill(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			OOMKilled: []string{"my-app-container"},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
	if corr.ResponsibleTeam != "DevOps" {
		t.Errorf("expected DevOps, got %q", corr.ResponsibleTeam)
	}
}

func TestAnalyze_DockerFailedContainer(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			FailedContainers: []string{"runner exited(1)"},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_DockerMixedFailures(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			OOMKilled:        []string{"oom-1", "oom-2"},
			FailedContainers: []string{"fail-1"},
		},
	}
	corr := Analyze(buildCtx, results)
	if len(corr.Evidence) != 3 {
		t.Errorf("expected 3 evidence items, got %d", len(corr.Evidence))
	}
}

func TestAnalyze_DockerHealthy(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure default, got %q", corr.RootCauseType)
	}
}

// --- Priority 5: JFrog Issues ---

func TestAnalyze_JFrogMissingArtifact(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"com.company:artifact:1.0.0"},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "DependencyIssue" {
		t.Errorf("expected DependencyIssue, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_JFrogAvailable(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: true,
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected fallthrough, got %q", corr.RootCauseType)
	}
}

// --- Priority 6: Nexus Issues ---

func TestAnalyze_NexusMissingArtifacts(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"org.example:core:2.0", "org.example:utils:2.0"},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "DependencyIssue" {
		t.Errorf("expected DependencyIssue, got %q", corr.RootCauseType)
	}
	if len(corr.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(corr.Evidence))
	}
}

func TestAnalyze_NexusAvailable(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: true,
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected fallthrough, got %q", corr.RootCauseType)
	}
}

// --- Priority 7: Default ---

func TestAnalyze_DefaultInfrastructure(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure default, got %q", corr.RootCauseType)
	}
	if corr.ResponsibleTeam != "DevOps" {
		t.Errorf("expected DevOps, got %q", corr.ResponsibleTeam)
	}
}

func TestAnalyze_NilResults(t *testing.T) {
	buildCtx := &models.BuildContext{}
	corr := Analyze(buildCtx, nil)
	if corr == nil {
		t.Fatal("expected non-nil correlation")
	}
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
}

// --- Priority Ordering Tests ---

func TestAnalyze_K8sBeatsDocker(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{
			OOMKills: []string{"k8s-oom"},
		},
		DockerResult: &models.DockerAgentResult{
			OOMKilled: []string{"docker-oom"},
		},
	}
	corr := Analyze(buildCtx, results)
	found := false
	for _, e := range corr.Evidence {
		if e == "OOM kill detected: k8s-oom" {
			found = true
		}
	}
	if !found {
		t.Error("expected K8s evidence (higher priority)")
	}
}

func TestAnalyze_DockerBeatsJFrog(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			FailedContainers: []string{"docker-fail"},
		},
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"artifact"},
		},
	}
	corr := Analyze(buildCtx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure (Docker), got %q", corr.RootCauseType)
	}
}

func TestAnalyze_JFrogBeatsNexus(t *testing.T) {
	buildCtx := &models.BuildContext{}
	results := &models.McpResults{
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"jfrog:art"},
		},
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"nexus:art"},
		},
	}
	corr := Analyze(buildCtx, results)
	found := false
	for _, e := range corr.Evidence {
		if e == "Missing artifact: jfrog:art" {
			found = true
		}
	}
	if !found {
		t.Error("expected JFrog evidence (higher priority)")
	}
}

// --- Helper Function Tests ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestExtractRepoFromJobName(t *testing.T) {
	tests := []struct {
		job  string
		want string
	}{
		{"Build - orders", "orders"},
		{"Build - payments", "payments"},
		{"payments Build", "payments"},
		{"Deploy - payments", ""},
		{"Build", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractRepoFromJobName(tt.job)
		if got != tt.want {
			t.Errorf("extractRepoFromJobName(%q) = %q, want %q", tt.job, got, tt.want)
		}
	}
}
