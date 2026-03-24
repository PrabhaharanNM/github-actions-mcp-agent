package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// dockerTestContainer is a local type matching the anonymous struct in parseContainerList.
type dockerTestContainer struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

// dockerTestInspect is a local type matching the anonymous struct in inspectContainer.
type dockerTestInspect struct {
	State struct {
		ExitCode  int    `json:"ExitCode"`
		OOMKilled bool   `json:"OOMKilled"`
		Error     string `json:"Error"`
	} `json:"State"`
	Config struct {
		Image string `json:"Image"`
	} `json:"Config"`
}

// dockerTestDiskUsage is a local type matching the anonymous struct in parseDiskUsage.
type dockerTestDiskUsage struct {
	LayersSize int64 `json:"LayersSize"`
	Images     []struct {
		Size       int64 `json:"Size"`
		SharedSize int64 `json:"SharedSize"`
	} `json:"Images"`
	Containers []struct {
		SizeRw     int64 `json:"SizeRw"`
		SizeRootFs int64 `json:"SizeRootFs"`
	} `json:"Containers"`
	Volumes []struct {
		UsageData struct {
			Size int64 `json:"Size"`
		} `json:"UsageData"`
	} `json:"Volumes"`
}

func TestDockerAgent_SkipWhenNoConfig(t *testing.T) {
	req := &models.AnalysisRequest{}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when docker not configured, got %+v", result)
	}
}

func TestDockerAgent_ListContainers(t *testing.T) {
	containers := []dockerTestContainer{
		{ID: "abc123def456", Names: []string{"/web-app"}, Image: "nginx:latest", State: "running", Status: "Up 2 hours"},
		{ID: "def456abc789", Names: []string{"/worker"}, Image: "python:3.9", State: "exited", Status: "Exited (1) 5 min ago"},
	}

	var inspectResp dockerTestInspect
	inspectResp.State.ExitCode = 1
	inspectResp.State.OOMKilled = false

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/def456abc789/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerTestDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// GHA DockerAgent uses doRequest for plain TCP, which creates its own http.Client.
	// By pointing the host at the test server, requests go to the mock.
	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ContainerStatuses) != 2 {
		t.Fatalf("expected 2 container statuses, got %d", len(result.ContainerStatuses))
	}
	if result.ContainerStatuses[0].Name != "web-app" {
		t.Errorf("expected first container name 'web-app', got %q", result.ContainerStatuses[0].Name)
	}
	if result.ContainerStatuses[0].State != "running" {
		t.Errorf("expected first container state 'running', got %q", result.ContainerStatuses[0].State)
	}
	if result.ContainerStatuses[1].Name != "worker" {
		t.Errorf("expected second container name 'worker', got %q", result.ContainerStatuses[1].Name)
	}
}

func TestDockerAgent_OOMKilledDetection(t *testing.T) {
	containers := []dockerTestContainer{
		{ID: "oom123container", Names: []string{"/oom-victim"}, Image: "java:11", State: "exited", Status: "Exited (137)"},
	}

	var inspectResp dockerTestInspect
	inspectResp.State.ExitCode = 137
	inspectResp.State.OOMKilled = true

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/oom123container/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerTestDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.OOMKilled) != 1 {
		t.Fatalf("expected 1 OOMKilled entry, got %d", len(result.OOMKilled))
	}
	if !strings.Contains(result.OOMKilled[0], "oom-victim") {
		t.Errorf("expected OOMKilled entry to contain 'oom-victim', got %q", result.OOMKilled[0])
	}
}

func TestDockerAgent_FailedContainers(t *testing.T) {
	containers := []dockerTestContainer{
		{ID: "dead123abc456", Names: []string{"/failed-svc"}, Image: "myapp:v1", State: "exited", Status: "Exited (1)"},
	}

	var inspectResp dockerTestInspect
	inspectResp.State.ExitCode = 1

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/dead123abc456/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerTestDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.FailedContainers) != 1 {
		t.Fatalf("expected 1 failed container, got %d", len(result.FailedContainers))
	}
	if !strings.Contains(result.FailedContainers[0], "failed-svc") {
		t.Errorf("expected failed container entry to contain 'failed-svc', got %q", result.FailedContainers[0])
	}
}

func TestDockerAgent_DiskUsage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]dockerTestContainer{})
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		// Build the JSON manually for the anonymous struct shapes.
		resp := map[string]interface{}{
			"LayersSize": 104857600, // 100 MB
			"Images": []map[string]interface{}{
				{"Size": 536870912, "SharedSize": 0},  // 512 MB
				{"Size": 1073741824, "SharedSize": 0}, // 1 GB
			},
			"Containers": []map[string]interface{}{
				{"SizeRw": 10485760, "SizeRootFs": 0},
			},
			"Volumes": []map[string]interface{}{
				{"UsageData": map[string]interface{}{"Size": 214748364}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DiskUsage == "" {
		t.Fatal("expected non-empty DiskUsage")
	}
	// The GHA version formats as "layers=XMB images=XMB volumes=XMB".
	if !strings.Contains(result.DiskUsage, "images=") {
		t.Errorf("expected DiskUsage to contain 'images=', got %q", result.DiskUsage)
	}
	if !strings.Contains(result.DiskUsage, "volumes=") {
		t.Errorf("expected DiskUsage to contain 'volumes=', got %q", result.DiskUsage)
	}
}

func TestDockerAgent_ImageIssues(t *testing.T) {
	containers := []dockerTestContainer{
		{ID: "imgfail123456", Names: []string{"/bad-image"}, Image: "nonexistent:latest", State: "dead", Status: "Dead"},
	}

	var inspectResp dockerTestInspect
	inspectResp.State.ExitCode = 125
	inspectResp.State.Error = "image pull failed: nonexistent:latest"
	inspectResp.Config.Image = "nonexistent:latest"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/imgfail123456/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerTestDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The GHA version checks if State.Error contains "image" AND Config.Image is set.
	if len(result.ImageIssues) != 1 {
		t.Fatalf("expected 1 image issue, got %d: %v", len(result.ImageIssues), result.ImageIssues)
	}
	if !strings.Contains(result.ImageIssues[0], "bad-image") {
		t.Errorf("expected image issue to reference 'bad-image', got %q", result.ImageIssues[0])
	}
}

func TestDockerAgent_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("expected graceful handling (no error returned), got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even on server errors")
	}
	if len(result.ContainerStatuses) != 0 {
		t.Errorf("expected empty ContainerStatuses on server error, got %d", len(result.ContainerStatuses))
	}
	if result.DiskUsage != "" {
		t.Errorf("expected empty DiskUsage on server error, got %q", result.DiskUsage)
	}
}
