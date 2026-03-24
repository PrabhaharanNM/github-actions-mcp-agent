package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// ---------------------------------------------------------------------------
// TestConvertModelId
// ---------------------------------------------------------------------------

func TestConvertModelId(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "claude-3-sonnet bedrock ID",
			input:    "anthropic.claude-3-sonnet-20240229-v1:0",
			expected: "claude-3-sonnet-20240229",
		},
		{
			name:     "claude-3-haiku bedrock ID",
			input:    "anthropic.claude-3-haiku-20240307-v1:0",
			expected: "claude-3-haiku-20240307",
		},
		{
			name:     "claude-3-opus bedrock ID",
			input:    "anthropic.claude-3-opus-20240229-v1:0",
			expected: "claude-3-opus-20240229",
		},
		{
			name:     "claude-3-5-sonnet bedrock ID",
			input:    "anthropic.claude-3-5-sonnet-20241022-v2:0",
			expected: "claude-3-5-sonnet-20241022",
		},
		{
			name:     "claude-sonnet-4 bedrock ID",
			input:    "anthropic.claude-sonnet-4-20250514-v1:0",
			expected: "claude-sonnet-4-20250514",
		},
		{
			name:     "unknown bedrock ID with anthropic prefix strips prefix and version",
			input:    "anthropic.claude-4-opus-20260101-v1:0",
			expected: "claude-4-opus-20260101",
		},
		{
			name:     "non-anthropic ID returned unchanged",
			input:    "claude-3-opus-20240229",
			expected: "claude-3-opus-20240229",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := convertModelId(tc.input)
			if got != tc.expected {
				t.Errorf("convertModelId(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestParseAnalysisResponse
// ---------------------------------------------------------------------------

func TestParseAnalysisResponse_DirectJSON(t *testing.T) {
	input := `{"category":"TestFailure","rootCauseSummary":"Unit test NPE","rootCauseDetails":"Null pointer in FooTest","evidence":["stack trace"],"nextSteps":["fix null check"],"confidence":"high"}`

	analysis, err := parseAnalysisResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Category != "TestFailure" {
		t.Errorf("Category = %q, want %q", analysis.Category, "TestFailure")
	}
	if analysis.RootCauseSummary != "Unit test NPE" {
		t.Errorf("RootCauseSummary = %q, want %q", analysis.RootCauseSummary, "Unit test NPE")
	}
	if analysis.Confidence != "high" {
		t.Errorf("Confidence = %q, want %q", analysis.Confidence, "high")
	}
	if len(analysis.Evidence) != 1 || analysis.Evidence[0] != "stack trace" {
		t.Errorf("Evidence = %v, want [\"stack trace\"]", analysis.Evidence)
	}
	if len(analysis.NextSteps) != 1 || analysis.NextSteps[0] != "fix null check" {
		t.Errorf("NextSteps = %v, want [\"fix null check\"]", analysis.NextSteps)
	}
}

func TestParseAnalysisResponse_JSONBlock(t *testing.T) {
	input := `Here is my analysis:
{"category":"Infrastructure","rootCauseSummary":"Disk full","rootCauseDetails":"Build agent ran out of disk","evidence":[],"nextSteps":[],"confidence":"medium"}
That concludes the analysis.`

	analysis, err := parseAnalysisResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Category != "Infrastructure" {
		t.Errorf("Category = %q, want %q", analysis.Category, "Infrastructure")
	}
	if analysis.Confidence != "medium" {
		t.Errorf("Confidence = %q, want %q", analysis.Confidence, "medium")
	}
}

func TestParseAnalysisResponse_NoJSON(t *testing.T) {
	input := "This is just plain text with no JSON at all."

	_, err := parseAnalysisResponse(input)
	if err == nil {
		t.Fatal("expected error for text with no JSON, got nil")
	}
	if !strings.Contains(err.Error(), "no valid JSON") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "no valid JSON")
	}
}

func TestParseAnalysisResponse_NestedBraces(t *testing.T) {
	input := `{"category":"CodeChange","rootCauseSummary":"Merge conflict","rootCauseDetails":"Conflicting changes in module: {\"file\":\"main.go\"}","evidence":["diff output"],"nextSteps":["resolve conflict"],"confidence":"high"}`

	analysis, err := parseAnalysisResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Category != "CodeChange" {
		t.Errorf("Category = %q, want %q", analysis.Category, "CodeChange")
	}
	if !strings.Contains(analysis.RootCauseDetails, "main.go") {
		t.Errorf("RootCauseDetails should contain nested object content, got %q", analysis.RootCauseDetails)
	}
}

// ---------------------------------------------------------------------------
// TestCallDirectAPI helpers
// ---------------------------------------------------------------------------

// newMockAnthropicServer creates an httptest server that validates request
// headers and body, then returns the given status code and response body.
func newMockAnthropicServer(t *testing.T, statusCode int, responseBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate method and path.
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected path /v1/messages, got %s", r.URL.Path)
		}

		// Validate required headers.
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key = %q, want %q", got, "test-key")
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q, want %q", got, "2023-06-01")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(responseBody))
	}))
}

// newTestRequest creates a minimal AnalysisRequest wired to the given mock
// server URL and API key.
func newTestRequest(serverURL, apiKey string) *models.AnalysisRequest {
	return &models.AnalysisRequest{
		AWS: models.AWSConfig{
			AnthropicBaseUrl: serverURL,
			AnthropicApiKey:  apiKey,
		},
	}
}

// ---------------------------------------------------------------------------
// TestCallDirectAPI
// ---------------------------------------------------------------------------

func TestCallDirectAPI_Success(t *testing.T) {
	server := newMockAnthropicServer(t, 200,
		`{"content":[{"type":"text","text":"analysis result"}]}`)
	defer server.Close()

	// Override the mock to also validate the request body structure.
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected path /v1/messages, got %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key = %q, want %q", got, "test-key")
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q, want %q", got, "2023-06-01")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		// Validate required fields in the request body.
		if _, ok := reqBody["model"]; !ok {
			t.Error("request body missing 'model' field")
		}
		if _, ok := reqBody["max_tokens"]; !ok {
			t.Error("request body missing 'max_tokens' field")
		}
		if _, ok := reqBody["temperature"]; !ok {
			t.Error("request body missing 'temperature' field")
		}
		if _, ok := reqBody["system"]; !ok {
			t.Error("request body missing 'system' field")
		}
		if _, ok := reqBody["messages"]; !ok {
			t.Error("request body missing 'messages' field")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"analysis result"}]}`))
	})

	req := newTestRequest(server.URL, "test-key")
	result, err := callDirectAPI(context.Background(), req, "system prompt", "user prompt", "claude-3-sonnet-20240229")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "analysis result" {
		t.Errorf("result = %q, want %q", result, "analysis result")
	}
}

func TestCallDirectAPI_ErrorResponse(t *testing.T) {
	server := newMockAnthropicServer(t, 400,
		`{"error":{"type":"invalid_request_error","message":"bad request"}}`)
	defer server.Close()

	req := newTestRequest(server.URL, "test-key")
	_, err := callDirectAPI(context.Background(), req, "system", "user", "claude-3-sonnet-20240229")
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to contain status code 400", err.Error())
	}
}

func TestCallDirectAPI_APIError(t *testing.T) {
	server := newMockAnthropicServer(t, 200,
		`{"error":{"type":"invalid_request_error","message":"bad request"},"content":[]}`)
	defer server.Close()

	req := newTestRequest(server.URL, "test-key")
	_, err := callDirectAPI(context.Background(), req, "system", "user", "claude-3-sonnet-20240229")
	if err == nil {
		t.Fatal("expected error for API error response, got nil")
	}
	if !strings.Contains(err.Error(), "invalid_request_error") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "invalid_request_error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "bad request")
	}
}

func TestCallDirectAPI_EmptyContent(t *testing.T) {
	server := newMockAnthropicServer(t, 200, `{"content":[]}`)
	defer server.Close()

	req := newTestRequest(server.URL, "test-key")
	_, err := callDirectAPI(context.Background(), req, "system", "user", "claude-3-sonnet-20240229")
	if err == nil {
		t.Fatal("expected error for empty content, got nil")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "empty response")
	}
}

func TestCallDirectAPI_ModelIdConversion(t *testing.T) {
	var receivedModel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		_ = json.Unmarshal(body, &reqBody)
		if m, ok := reqBody["model"].(string); ok {
			receivedModel = m
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()

	req := newTestRequest(server.URL, "test-key")

	// Pass a bedrock model ID and verify it gets converted for the direct API call.
	_, err := callDirectAPI(context.Background(), req, "system", "user", "anthropic.claude-3-sonnet-20240229-v1:0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedModel != "claude-3-sonnet-20240229" {
		t.Errorf("model sent to API = %q, want %q", receivedModel, "claude-3-sonnet-20240229")
	}
}

func TestCallDirectAPI_CustomBaseUrl(t *testing.T) {
	var requestReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()

	req := newTestRequest(server.URL, "test-key")

	_, err := callDirectAPI(context.Background(), req, "system", "user", "claude-3-sonnet-20240229")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !requestReceived {
		t.Error("expected request to be sent to custom base URL server, but it was not received")
	}
}

// ---------------------------------------------------------------------------
// TestLoadSystemPrompt
// ---------------------------------------------------------------------------

func TestLoadSystemPrompt_EmptyEmbedded(t *testing.T) {
	// In the test binary, embeddedPrompt is populated via //go:embed only when
	// rca_prompt.txt exists at build time. Save and restore the original value
	// to avoid side effects on other tests.
	original := embeddedPrompt
	defer func() { embeddedPrompt = original }()

	embeddedPrompt = ""

	_, err := loadSystemPrompt()
	if err == nil {
		t.Fatal("expected error when embeddedPrompt is empty, got nil")
	}
	if !strings.Contains(err.Error(), "embedded prompt is empty") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "embedded prompt is empty")
	}
}

// ---------------------------------------------------------------------------
// TestAnalyze_FallbackOnParseError
// ---------------------------------------------------------------------------

func TestAnalyze_FallbackOnParseError(t *testing.T) {
	// Set up a mock server that returns non-JSON text so the parse step fails
	// and Analyze falls back to a basic analysis.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"This is plain text, not JSON."}]}`))
	}))
	defer server.Close()

	req := &models.AnalysisRequest{
		AWS: models.AWSConfig{
			Provider:         "direct",
			AnthropicBaseUrl: server.URL,
			AnthropicApiKey:  "test-key",
			ModelId:          "claude-3-sonnet-20240229",
		},
	}

	analysis, err := Analyze(
		context.Background(),
		req,
		&models.BuildContext{},
		&models.McpResults{},
		&models.Correlation{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.Category != "Unknown" {
		t.Errorf("Category = %q, want %q", analysis.Category, "Unknown")
	}
	if analysis.Confidence != "low" {
		t.Errorf("Confidence = %q, want %q", analysis.Confidence, "low")
	}
	if analysis.RootCauseSummary != "This is plain text, not JSON." {
		t.Errorf("RootCauseSummary = %q, want raw text from API", analysis.RootCauseSummary)
	}
	if analysis.RootCauseDetails != "This is plain text, not JSON." {
		t.Errorf("RootCauseDetails = %q, want raw text from API", analysis.RootCauseDetails)
	}
}
