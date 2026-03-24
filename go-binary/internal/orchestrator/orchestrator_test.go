package orchestrator

import (
	"context"
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// TestRunAgentsParallel_CategoryBitBucket verifies that selecting "bitbucket"
// as repoSoftware runs only the BitBucket agent (not GitHub).
func TestRunAgentsParallel_CategoryBitBucket(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			RepoSoftware: "bitbucket",
		},
		BitBucket: models.BitBucketConfig{Url: "http://localhost:1", Username: "u", Password: "p"},
	}
	buildCtx := &models.BuildContext{Repo: "proj/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// GitHub should NOT have been run
	if results.GithubResult != nil {
		t.Error("GitHub agent should not run when repoSoftware=bitbucket")
	}
}

// TestRunAgentsParallel_CategoryGitHubDefault verifies that when repoSoftware
// is "github" or empty, the GitHub agent runs (GHA default).
func TestRunAgentsParallel_CategoryGitHubDefault(t *testing.T) {
	// Explicit "github"
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			RepoSoftware: "github",
		},
		GithubToken: "fake-token",
		Owner:       "org",
		Repo:        "repo",
	}
	buildCtx := &models.BuildContext{Repo: "org/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.BitBucketResult != nil {
		t.Error("BitBucket agent should not run when repoSoftware=github")
	}
}

// TestRunAgentsParallel_EmptyRepoSoftwareDefaultsToGitHub verifies that on GHA,
// empty repoSoftware defaults to GitHub.
func TestRunAgentsParallel_EmptyRepoSoftwareDefaultsToGitHub(t *testing.T) {
	req := &models.AnalysisRequest{
		GithubToken: "fake-token",
		Owner:       "org",
		Repo:        "repo",
	}
	buildCtx := &models.BuildContext{Repo: "org/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// On GHA, empty repoSoftware defaults to github
	if results.BitBucketResult != nil {
		t.Error("BitBucket should not run when repoSoftware is empty on GHA")
	}
}

// TestRunAgentsParallel_CategoryDocker verifies that selecting "docker"
// runs only the Docker agent.
func TestRunAgentsParallel_CategoryDocker(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ClusterType: "docker",
		},
		Docker: models.DockerConfig{Host: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{Repo: "org/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.KubernetesResult != nil {
		t.Error("K8s agent should not run when clusterType=docker")
	}
}

// TestRunAgentsParallel_CategoryKubernetes verifies that selecting "kubernetes"
// runs only the Kubernetes agent.
func TestRunAgentsParallel_CategoryKubernetes(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ClusterType: "kubernetes",
		},
		Kubernetes: models.KubernetesConfig{ApiUrl: "http://localhost:1", Token: "fake"},
	}
	buildCtx := &models.BuildContext{Repo: "org/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.DockerResult != nil {
		t.Error("Docker agent should not run when clusterType=kubernetes")
	}
}

// TestRunAgentsParallel_CategoryNexus verifies that selecting "nexus"
// runs only the Nexus agent.
func TestRunAgentsParallel_CategoryNexus(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ArtifactManager: "nexus",
		},
		Nexus: models.NexusConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{Repo: "org/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.JFrogResult != nil {
		t.Error("JFrog agent should not run when artifactManager=nexus")
	}
}

// TestRunAgentsParallel_CategoryJFrog verifies that selecting "jfrog"
// runs only the JFrog agent.
func TestRunAgentsParallel_CategoryJFrog(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ArtifactManager: "jfrog",
		},
		JFrog: models.JFrogConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{Repo: "org/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.NexusResult != nil {
		t.Error("Nexus agent should not run when artifactManager=jfrog")
	}
}

// TestRunAgentsParallel_AutoDetectClusterAndArtifact verifies auto-detect when
// no categories are set but configs are present.
func TestRunAgentsParallel_AutoDetectClusterAndArtifact(t *testing.T) {
	req := &models.AnalysisRequest{
		GithubToken: "token",
		Owner:       "org",
		Repo:        "repo",
		// No categories — auto-detect for cluster/artifact
		Docker: models.DockerConfig{Host: "http://localhost:1"},
		Nexus:  models.NexusConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{Repo: "org/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// K8s should NOT run (no API URL)
	if results.KubernetesResult != nil {
		t.Error("K8s agent should not run without config in auto-detect")
	}
	// JFrog should NOT run (no URL)
	if results.JFrogResult != nil {
		t.Error("JFrog agent should not run without config in auto-detect")
	}
}

// TestRunAgentsParallel_CaseInsensitive verifies category values are case-insensitive.
func TestRunAgentsParallel_CaseInsensitive(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			RepoSoftware:    "BITBUCKET",
			ClusterType:     "Docker",
			ArtifactManager: "JFROG",
		},
		BitBucket: models.BitBucketConfig{Url: "http://localhost:1", Username: "u", Password: "p"},
		Docker:    models.DockerConfig{Host: "http://localhost:1"},
		JFrog:     models.JFrogConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{Repo: "proj/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.GithubResult != nil {
		t.Error("GitHub should not run when repoSoftware=BITBUCKET")
	}
	if results.KubernetesResult != nil {
		t.Error("K8s should not run when clusterType=Docker")
	}
	if results.NexusResult != nil {
		t.Error("Nexus should not run when artifactManager=JFROG")
	}
}

// TestRunAgentsParallel_EmptyConfig verifies no panics with entirely empty config.
func TestRunAgentsParallel_EmptyConfig(t *testing.T) {
	req := &models.AnalysisRequest{}
	buildCtx := &models.BuildContext{}

	// Should not panic
	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results == nil {
		t.Error("results should never be nil")
	}
}

// TestRunAgentsParallel_FullCrossStack verifies all three alternative agents
// (BitBucket, Docker, Nexus) are selected together.
func TestRunAgentsParallel_FullCrossStack(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			RepoSoftware:    "bitbucket",
			ClusterType:     "docker",
			ArtifactManager: "nexus",
		},
		BitBucket: models.BitBucketConfig{Url: "http://localhost:1", Username: "u", Password: "p"},
		Docker:    models.DockerConfig{Host: "http://localhost:1"},
		Nexus:     models.NexusConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{Repo: "proj/repo"}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// Verify only alternative agents were selected
	if results.GithubResult != nil {
		t.Error("GitHub should not run when bitbucket is selected")
	}
	if results.KubernetesResult != nil {
		t.Error("K8s should not run when docker is selected")
	}
	if results.JFrogResult != nil {
		t.Error("JFrog should not run when nexus is selected")
	}
}
