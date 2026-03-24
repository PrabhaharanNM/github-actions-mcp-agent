package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// resultsDir returns the base directory for storing analysis results.
func resultsDir() string {
	return filepath.Join(os.TempDir(), "mcp-results")
}

// resultFilePath returns the full path for an analysis result JSON file.
func resultFilePath(analysisID string) string {
	return filepath.Join(resultsDir(), analysisID+".json")
}

// statusFilePath returns the full path for an analysis status file.
func statusFilePath(analysisID string) string {
	return filepath.Join(resultsDir(), analysisID+".status")
}

// Save persists an AnalysisResult to disk as JSON.
func Save(analysisID string, result *models.AnalysisResult) error {
	dir := resultsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[STORAGE] Failed to create directory %s: %v", dir, err)
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("[STORAGE] Failed to marshal analysis result: %v", err)
		return fmt.Errorf("failed to marshal analysis result: %w", err)
	}

	path := resultFilePath(analysisID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[STORAGE] Failed to write result file %s: %v", path, err)
		return fmt.Errorf("failed to write result file: %w", err)
	}

	log.Printf("[STORAGE] Saved analysis result: %s", path)
	return nil
}

// Load reads an AnalysisResult from disk by its analysis ID.
func Load(analysisID string) (*models.AnalysisResult, error) {
	path := resultFilePath(analysisID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[STORAGE] Analysis result not found: %s", path)
			return nil, fmt.Errorf("analysis result not found: %s", analysisID)
		}
		log.Printf("[STORAGE] Failed to read result file %s: %v", path, err)
		return nil, fmt.Errorf("failed to read result file: %w", err)
	}

	var result models.AnalysisResult
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("[STORAGE] Failed to unmarshal result file %s: %v", path, err)
		return nil, fmt.Errorf("failed to unmarshal result file: %w", err)
	}

	log.Printf("[STORAGE] Loaded analysis result: %s", path)
	return &result, nil
}

// SaveStatus writes a simple status string for a given analysis ID.
func SaveStatus(analysisID, status string) error {
	dir := resultsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[STORAGE] Failed to create directory %s: %v", dir, err)
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	path := statusFilePath(analysisID)
	if err := os.WriteFile(path, []byte(status), 0644); err != nil {
		log.Printf("[STORAGE] Failed to write status file %s: %v", path, err)
		return fmt.Errorf("failed to write status file: %w", err)
	}

	log.Printf("[STORAGE] Saved status '%s' for analysis: %s", status, analysisID)
	return nil
}
