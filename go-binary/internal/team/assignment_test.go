package team

import (
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

func TestAssign_InfrastructureGoesToDevOps(t *testing.T) {
	req := &models.AnalysisRequest{}
	buildCtx := &models.BuildContext{}
	corr := &models.Correlation{
		IsInfrastructure: true,
	}
	mgr := Assign(req, buildCtx, corr)
	if mgr.Name != "DevOps Team" {
		t.Errorf("expected DevOps Team, got %q", mgr.Name)
	}
}

func TestAssign_ResponsibleRepoMatchesMapping(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings: `{"myapp": {"name": "App Team", "email": "app@example.com", "jira_username": "app-lead"}}`,
	}
	buildCtx := &models.BuildContext{}
	corr := &models.Correlation{
		ResponsibleRepository: "myapp",
	}
	mgr := Assign(req, buildCtx, corr)
	if mgr.Name != "App Team" {
		t.Errorf("expected App Team, got %q", mgr.Name)
	}
}

func TestAssign_FailedJobExtraction(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings: `{"orders": {"name": "Orders Team", "email": "orders@example.com"}}`,
	}
	buildCtx := &models.BuildContext{
		FailedJob: "Build - orders",
	}
	corr := &models.Correlation{}
	mgr := Assign(req, buildCtx, corr)
	if mgr.Name != "Orders Team" {
		t.Errorf("expected Orders Team, got %q", mgr.Name)
	}
}

func TestAssign_FallbackToDevOps(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings: `{"other": {"name": "Other Team", "email": "other@example.com"}}`,
	}
	buildCtx := &models.BuildContext{}
	corr := &models.Correlation{}
	mgr := Assign(req, buildCtx, corr)
	if mgr.Name != "DevOps Team" {
		t.Errorf("expected DevOps Team fallback, got %q", mgr.Name)
	}
}

func TestAssign_CustomDevopsManager(t *testing.T) {
	req := &models.AnalysisRequest{
		DevopsManager: `{"name": "Custom DevOps", "email": "custom@example.com"}`,
	}
	buildCtx := &models.BuildContext{}
	corr := &models.Correlation{
		IsInfrastructure: true,
	}
	mgr := Assign(req, buildCtx, corr)
	if mgr.Name != "Custom DevOps" {
		t.Errorf("expected Custom DevOps, got %q", mgr.Name)
	}
}
