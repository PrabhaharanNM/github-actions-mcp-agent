package agents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

func TestBitBucketAgent_SkipWhenNoConfig(t *testing.T) {
	req := &models.AnalysisRequest{}
	agent := NewBitBucketAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{Repo: "proj/repo"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when bitbucket not configured, got %+v", result)
	}
}

func TestBitBucketAgent_FetchCommits(t *testing.T) {
	commitsResp := map[string]interface{}{
		"values": []map[string]interface{}{
			{
				"id":      "abc123def",
				"message": "fix: resolve race condition",
				"author": map[string]interface{}{
					"name":         "Developer One",
					"emailAddress": "dev1@example.com",
				},
				"authorTimestamp": 1705305600000,
			},
			{
				"id":      "def456abc",
				"message": "feat: add retry logic",
				"author": map[string]interface{}{
					"name":         "Developer Two",
					"emailAddress": "dev2@example.com",
				},
				"authorTimestamp": 1705219200000,
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/projects/MYPROJ/repos/myrepo/commits", func(w http.ResponseWriter, r *http.Request) {
		// Only respond to the list endpoint (no SHA suffix).
		if r.URL.Query().Get("limit") != "" {
			json.NewEncoder(w).Encode(commitsResp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	// CODEOWNERS paths - return 404 so they are skipped.
	mux.HandleFunc("/rest/api/1.0/projects/MYPROJ/repos/myrepo/browse/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		BitBucket: models.BitBucketConfig{
			Url:      server.URL,
			Username: "admin",
			Password: "secret",
		},
	}
	agent := NewBitBucketAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{Repo: "MYPROJ/myrepo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.RecentCommits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(result.RecentCommits))
	}
	if result.RecentCommits[0].SHA != "abc123def" {
		t.Errorf("expected first commit SHA 'abc123def', got %q", result.RecentCommits[0].SHA)
	}
	if result.RecentCommits[0].Author != "Developer One" {
		t.Errorf("expected first commit author 'Developer One', got %q", result.RecentCommits[0].Author)
	}
	if result.RecentCommits[1].SHA != "def456abc" {
		t.Errorf("expected second commit SHA 'def456abc', got %q", result.RecentCommits[1].SHA)
	}
}

func TestBitBucketAgent_FetchChangedFiles(t *testing.T) {
	commitsResp := map[string]interface{}{
		"values": []map[string]interface{}{
			{
				"id":      "sha123",
				"message": "update",
				"author": map[string]interface{}{
					"name":         "Dev",
					"emailAddress": "dev@example.com",
				},
				"authorTimestamp": 1705305600000,
			},
		},
	}

	changesResp := map[string]interface{}{
		"values": []map[string]interface{}{
			{"path": map[string]interface{}{"toString": "src/main.go"}},
			{"path": map[string]interface{}{"toString": "src/handler.go"}},
			{"path": map[string]interface{}{"toString": "README.md"}},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/repo/commits", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "" {
			json.NewEncoder(w).Encode(commitsResp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/repo/commits/sha123/changes", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(changesResp)
	})
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/repo/browse/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		BitBucket: models.BitBucketConfig{
			Url:      server.URL,
			Username: "admin",
			Password: "secret",
		},
	}
	agent := NewBitBucketAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{
		Repo: "PROJ/repo",
		SHA:  "sha123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ChangedFiles) != 3 {
		t.Fatalf("expected 3 changed files, got %d", len(result.ChangedFiles))
	}
	expected := []string{"src/main.go", "src/handler.go", "README.md"}
	for i, f := range expected {
		if result.ChangedFiles[i] != f {
			t.Errorf("changed file[%d] = %q, want %q", i, result.ChangedFiles[i], f)
		}
	}
}

func TestBitBucketAgent_FetchCodeOwners(t *testing.T) {
	codeownersResp := map[string]interface{}{
		"lines": []map[string]interface{}{
			{"text": "* @team-leads"},
			{"text": "/src/ @backend-team"},
		},
	}

	commitsResp := map[string]interface{}{
		"values": []interface{}{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/repo/commits", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(commitsResp)
	})
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/repo/browse/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(codeownersResp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		BitBucket: models.BitBucketConfig{
			Url:      server.URL,
			Username: "admin",
			Password: "secret",
		},
	}
	agent := NewBitBucketAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{Repo: "PROJ/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CodeOwners == "" {
		t.Fatal("expected non-empty CodeOwners")
	}
	if !strings.Contains(result.CodeOwners, "@team-leads") {
		t.Errorf("expected CodeOwners to contain '@team-leads', got %q", result.CodeOwners)
	}
	if !strings.Contains(result.CodeOwners, "@backend-team") {
		t.Errorf("expected CodeOwners to contain '@backend-team', got %q", result.CodeOwners)
	}
}

func TestBitBucketAgent_SplitRepo(t *testing.T) {
	agent := &BitBucketAgent{}

	tests := []struct {
		name            string
		repoPath        string
		expectedProject string
		expectedRepo    string
	}{
		{name: "normal split", repoPath: "project/repo", expectedProject: "project", expectedRepo: "repo"},
		{name: "no slash", repoPath: "noslash", expectedProject: "", expectedRepo: ""},
		{name: "empty string", repoPath: "", expectedProject: "", expectedRepo: ""},
		{name: "multiple slashes", repoPath: "org/sub/repo", expectedProject: "org", expectedRepo: "sub/repo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proj, repo := agent.splitRepo(tc.repoPath)
			if proj != tc.expectedProject {
				t.Errorf("splitRepo(%q) project = %q, want %q", tc.repoPath, proj, tc.expectedProject)
			}
			if repo != tc.expectedRepo {
				t.Errorf("splitRepo(%q) repo = %q, want %q", tc.repoPath, repo, tc.expectedRepo)
			}
		})
	}
}
