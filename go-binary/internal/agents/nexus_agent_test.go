package agents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

func TestNexusAgent_SkipWhenNoConfig(t *testing.T) {
	req := &models.AnalysisRequest{}
	agent := NewNexusAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{Repo: "owner/myrepo"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when nexus not configured, got %+v", result)
	}
}

func TestNexusAgent_HealthyWithArtifact(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/service/rest/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})
	mux.HandleFunc("/service/rest/v1/search", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name != "myrepo" {
			t.Errorf("expected search for 'myrepo', got %q", name)
		}
		resp := map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "repository": "maven-releases", "name": "myrepo", "version": "1.0.0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Nexus: models.NexusConfig{
			Url:      server.URL,
			Username: "admin",
			Password: "pass",
		},
	}
	agent := NewNexusAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{Repo: "owner/myrepo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RepositoryStatus != "healthy" {
		t.Errorf("expected RepositoryStatus 'healthy', got %q", result.RepositoryStatus)
	}
	if !result.ArtifactsAvailable {
		t.Error("expected ArtifactsAvailable=true")
	}
	if len(result.MissingArtifacts) != 0 {
		t.Errorf("expected no missing artifacts, got %v", result.MissingArtifacts)
	}
}

func TestNexusAgent_UnhealthyStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/service/rest/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	})
	mux.HandleFunc("/service/rest/v1/search", func(w http.ResponseWriter, r *http.Request) {
		// Even if search works, the status was unhealthy.
		resp := map[string]interface{}{"items": []interface{}{}}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Nexus: models.NexusConfig{
			Url:      server.URL,
			Username: "admin",
			Password: "pass",
		},
	}
	agent := NewNexusAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{Repo: "owner/myrepo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RepositoryStatus == "healthy" {
		t.Error("expected unhealthy RepositoryStatus")
	}
	if result.ArtifactsAvailable {
		t.Error("expected ArtifactsAvailable=false when status is unhealthy")
	}
}

func TestNexusAgent_ArtifactNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/service/rest/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})
	mux.HandleFunc("/service/rest/v1/search", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{"items": []interface{}{}}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Nexus: models.NexusConfig{
			Url:      server.URL,
			Username: "admin",
			Password: "pass",
		},
	}
	agent := NewNexusAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{Repo: "owner/myrepo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ArtifactsAvailable {
		t.Error("expected ArtifactsAvailable=false when artifact not found")
	}
	if len(result.MissingArtifacts) == 0 {
		t.Error("expected at least one missing artifact entry")
	}
}

func TestNexusAgent_DeriveArtifactName(t *testing.T) {
	agent := &NexusAgent{}

	tests := []struct {
		name     string
		buildCtx *models.BuildContext
		expected string
	}{
		{
			name:     "repo with owner",
			buildCtx: &models.BuildContext{Repo: "owner/myrepo"},
			expected: "myrepo",
		},
		{
			name:     "repo without owner",
			buildCtx: &models.BuildContext{Repo: "single-repo"},
			expected: "single-repo",
		},
		{
			name:     "workflow fallback",
			buildCtx: &models.BuildContext{Repo: "", Workflow: "build-workflow"},
			expected: "build-workflow",
		},
		{
			name:     "empty context",
			buildCtx: &models.BuildContext{},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := agent.deriveArtifactName(tc.buildCtx)
			if got != tc.expected {
				t.Errorf("deriveArtifactName() = %q, want %q", got, tc.expected)
			}
		})
	}
}
