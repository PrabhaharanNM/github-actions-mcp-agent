package integrations

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// FailureRecord tracks a single build failure for MTTR calculation.
type FailureRecord struct {
	Owner      string     `json:"owner"`
	Repo       string     `json:"repo"`
	Workflow   string     `json:"workflow"`
	RunID      int64      `json:"runId"`
	RunNumber  int        `json:"runNumber"`
	FailedAt   time.Time  `json:"failedAt"`
	Category   string     `json:"category"`
	Team       string     `json:"team"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	MTTR       string     `json:"mttr,omitempty"`
}

// mttrRecordsFile returns the path to the MTTR records JSON file.
func mttrRecordsFile() string {
	return filepath.Join(os.TempDir(), "mcp-results", "mttr-records.json")
}

// TrackMTTR records a build failure and computes rolling MTTR averages.
func TrackMTTR(req *models.AnalysisRequest, analysis *models.ClaudeAnalysis, teamMgr *models.TeamManager, ctx *models.BuildContext) error {
	recordsPath := mttrRecordsFile()

	// Ensure directory exists
	dir := filepath.Dir(recordsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[MTTR] Failed to create directory %s: %v", dir, err)
		return fmt.Errorf("failed to create mttr directory: %w", err)
	}

	// Load existing records
	records, err := loadRecords(recordsPath)
	if err != nil {
		log.Printf("[MTTR] Failed to load existing records, starting fresh: %v", err)
		records = []FailureRecord{}
	}

	// Determine team name
	team := ""
	if teamMgr != nil {
		team = teamMgr.Name
	}

	// Append new failure record with GitHub-specific fields
	newRecord := FailureRecord{
		Owner:     ctx.Owner,
		Repo:      ctx.Repo,
		Workflow:  ctx.Workflow,
		RunID:     ctx.RunID,
		RunNumber: ctx.RunNumber,
		FailedAt:  time.Now(),
		Category:  analysis.Category,
		Team:      team,
	}
	records = append(records, newRecord)

	// Calculate rolling averages
	now := time.Now()
	avg7 := calculateRollingAverage(records, now, 7*24*time.Hour)
	avg30 := calculateRollingAverage(records, now, 30*24*time.Hour)

	log.Printf("[MTTR] New failure recorded: %s/%s workflow=%s run=#%d (%s)", ctx.Owner, ctx.Repo, ctx.Workflow, ctx.RunNumber, analysis.Category)
	log.Printf("[MTTR] Total records: %d", len(records))
	if avg7 > 0 {
		log.Printf("[MTTR] 7-day rolling average MTTR: %s", avg7)
	} else {
		log.Println("[MTTR] 7-day rolling average MTTR: no resolved failures in window")
	}
	if avg30 > 0 {
		log.Printf("[MTTR] 30-day rolling average MTTR: %s", avg30)
	} else {
		log.Println("[MTTR] 30-day rolling average MTTR: no resolved failures in window")
	}

	// Save updated records
	if err := saveRecords(recordsPath, records); err != nil {
		log.Printf("[MTTR] Failed to save records: %v", err)
		return fmt.Errorf("failed to save mttr records: %w", err)
	}

	return nil
}

// loadRecords reads FailureRecords from the JSON file.
func loadRecords(path string) ([]FailureRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []FailureRecord{}, nil
		}
		return nil, err
	}

	var records []FailureRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mttr records: %w", err)
	}
	return records, nil
}

// saveRecords writes FailureRecords to the JSON file.
func saveRecords(path string, records []FailureRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mttr records: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// calculateRollingAverage computes the average MTTR for resolved failures within
// the given time window from now.
func calculateRollingAverage(records []FailureRecord, now time.Time, window time.Duration) time.Duration {
	cutoff := now.Add(-window)
	var totalDuration time.Duration
	count := 0

	for _, r := range records {
		if r.ResolvedAt == nil {
			continue
		}
		if r.FailedAt.Before(cutoff) {
			continue
		}
		mttr := r.ResolvedAt.Sub(r.FailedAt)
		totalDuration += mttr
		count++
	}

	if count == 0 {
		return 0
	}
	return totalDuration / time.Duration(count)
}
