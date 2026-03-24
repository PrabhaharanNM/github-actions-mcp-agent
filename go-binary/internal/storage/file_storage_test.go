package storage

import (
	"os"
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

func TestSaveAndLoad(t *testing.T) {
	result := &models.AnalysisResult{
		Status:           "completed",
		Category:         "CodeChange",
		RootCauseSummary: "Test failure in API module",
	}

	analysisID := "test-save-load-001"
	defer os.Remove(resultFilePath(analysisID))

	if err := Save(analysisID, result); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(analysisID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Category != "CodeChange" {
		t.Errorf("expected category CodeChange, got %q", loaded.Category)
	}
	if loaded.RootCauseSummary != "Test failure in API module" {
		t.Errorf("expected root cause summary, got %q", loaded.RootCauseSummary)
	}
}

func TestLoad_NotFound(t *testing.T) {
	_, err := Load("nonexistent-analysis-id")
	if err == nil {
		t.Error("expected error for nonexistent analysis")
	}
}

func TestSaveStatus(t *testing.T) {
	analysisID := "test-status-001"
	defer os.Remove(statusFilePath(analysisID))

	if err := SaveStatus(analysisID, "in-progress"); err != nil {
		t.Fatalf("SaveStatus failed: %v", err)
	}

	data, err := os.ReadFile(statusFilePath(analysisID))
	if err != nil {
		t.Fatalf("failed to read status file: %v", err)
	}
	if string(data) != "in-progress" {
		t.Errorf("expected 'in-progress', got %q", string(data))
	}
}
