package claude

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

//go:embed rca_prompt.txt
var embeddedPrompt string

// bedrockRequest is the request payload sent to the Bedrock InvokeModel API
// for Anthropic Claude models.
type bedrockRequest struct {
	AnthropicVersion string           `json:"anthropic_version"`
	MaxTokens        int              `json:"max_tokens"`
	Temperature      float64          `json:"temperature"`
	System           string           `json:"system,omitempty"`
	Messages         []bedrockMessage `json:"messages"`
}

// bedrockMessage represents a single message in the Bedrock conversation.
type bedrockMessage struct {
	Role    string                `json:"role"`
	Content []bedrockContentPart `json:"content"`
}

// bedrockContentPart is a single content block within a message.
type bedrockContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// bedrockResponse is the response payload from the Bedrock InvokeModel API.
type bedrockResponse struct {
	Content []bedrockContentPart `json:"content"`
}

// Analyze sends the build context, MCP results, and correlation analysis to
// Claude and returns a structured analysis. Supports three providers:
// "bedrock" (default), "direct" (Anthropic API), "max" (Claude Max/Teams).
func Analyze(ctx context.Context, req *models.AnalysisRequest, buildCtx *models.BuildContext, mcpResults *models.McpResults, corr *models.Correlation) (*models.ClaudeAnalysis, error) {
	systemPrompt, err := loadSystemPrompt()
	if err != nil {
		log.Printf("warning: could not load system prompt, using default: %v", err)
		systemPrompt = defaultSystemPrompt
	}

	userPrompt := BuildUserPrompt(buildCtx, mcpResults, corr)

	modelId := req.AWS.ModelId
	if modelId == "" {
		modelId = "anthropic.claude-3-sonnet-20240229-v1:0"
	}

	provider := strings.ToLower(req.AWS.Provider)
	var rawText string

	switch provider {
	case "direct", "max":
		rawText, err = callDirectAPI(ctx, req, systemPrompt, userPrompt, modelId)
	default:
		rawText, err = callBedrock(ctx, req, systemPrompt, userPrompt, modelId)
	}
	if err != nil {
		return nil, err
	}

	analysis, err := parseAnalysisResponse(rawText)
	if err != nil {
		log.Printf("warning: could not parse Claude response as JSON, creating basic analysis: %v", err)
		analysis = &models.ClaudeAnalysis{
			Category:         "Unknown",
			RootCauseSummary: rawText,
			RootCauseDetails: rawText,
			Confidence:       "low",
		}
	}

	return analysis, nil
}

// callBedrock invokes Claude via AWS Bedrock with static credentials.
func callBedrock(ctx context.Context, req *models.AnalysisRequest, systemPrompt, userPrompt, modelId string) (string, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(req.AWS.Region),
	}

	if req.AWS.AccessKeyId != "" && req.AWS.SecretAccessKey != "" {
		staticCreds := credentials.NewStaticCredentialsProvider(
			req.AWS.AccessKeyId, req.AWS.SecretAccessKey, req.AWS.SessionToken,
		)
		opts = append(opts, awsconfig.WithCredentialsProvider(staticCreds))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	var brOpts []func(*bedrockruntime.Options)
	if req.AWS.VpcEndpoint != "" {
		brOpts = append(brOpts, func(o *bedrockruntime.Options) {
			o.BaseEndpoint = aws.String(req.AWS.VpcEndpoint)
		})
	}

	client := bedrockruntime.NewFromConfig(cfg, brOpts...)

	payload := bedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        4096,
		Temperature:      0.3,
		System:           systemPrompt,
		Messages: []bedrockMessage{
			{Role: "user", Content: []bedrockContentPart{{Type: "text", Text: userPrompt}}},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Bedrock request: %w", err)
	}

	output, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelId),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        payloadBytes,
	})
	if err != nil {
		return "", fmt.Errorf("Bedrock InvokeModel failed: %w", err)
	}

	var resp bedrockResponse
	if err := json.Unmarshal(output.Body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse Bedrock response: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from Bedrock")
	}
	return resp.Content[0].Text, nil
}

// callDirectAPI invokes Claude via the Anthropic Messages API (direct or max).
func callDirectAPI(ctx context.Context, req *models.AnalysisRequest, systemPrompt, userPrompt, modelId string) (string, error) {
	apiModelId := convertModelId(modelId)

	baseUrl := "https://api.anthropic.com"
	if req.AWS.AnthropicBaseUrl != "" {
		baseUrl = strings.TrimRight(req.AWS.AnthropicBaseUrl, "/")
	}

	type contentPart struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type message struct {
		Role    string        `json:"role"`
		Content []contentPart `json:"content"`
	}
	type apiRequest struct {
		Model       string    `json:"model"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float64   `json:"temperature"`
		System      string    `json:"system,omitempty"`
		Messages    []message `json:"messages"`
	}
	type apiError struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	type apiResponse struct {
		Content []contentPart `json:"content"`
		Error   *apiError     `json:"error,omitempty"`
	}

	payload := apiRequest{
		Model:       apiModelId,
		MaxTokens:   4096,
		Temperature: 0.3,
		System:      systemPrompt,
		Messages:    []message{{Role: "user", Content: []contentPart{{Type: "text", Text: userPrompt}}}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal API request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseUrl+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create API request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", req.AWS.AnthropicApiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("Anthropic API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read API response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Anthropic API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var ar apiResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}
	if ar.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s: %s", ar.Error.Type, ar.Error.Message)
	}
	if len(ar.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic API")
	}
	return ar.Content[0].Text, nil
}

// convertModelId converts Bedrock model IDs to Anthropic API model IDs.
func convertModelId(bedrockId string) string {
	mapping := map[string]string{
		"anthropic.claude-3-sonnet-20240229-v1:0":   "claude-3-sonnet-20240229",
		"anthropic.claude-3-haiku-20240307-v1:0":    "claude-3-haiku-20240307",
		"anthropic.claude-3-opus-20240229-v1:0":     "claude-3-opus-20240229",
		"anthropic.claude-3-5-sonnet-20241022-v2:0": "claude-3-5-sonnet-20241022",
		"anthropic.claude-sonnet-4-20250514-v1:0":   "claude-sonnet-4-20250514",
	}
	if mapped, ok := mapping[bedrockId]; ok {
		return mapped
	}
	if !strings.HasPrefix(bedrockId, "anthropic.") {
		return bedrockId
	}
	id := strings.TrimPrefix(bedrockId, "anthropic.")
	if idx := strings.LastIndex(id, "-v"); idx > 0 {
		id = id[:idx]
	}
	return id
}

// parseAnalysisResponse attempts to extract a ClaudeAnalysis from the model's
// text response. It tries direct JSON parsing first, then looks for a JSON
// block within the text.
func parseAnalysisResponse(text string) (*models.ClaudeAnalysis, error) {
	var analysis models.ClaudeAnalysis

	// Try direct JSON parse.
	if err := json.Unmarshal([]byte(text), &analysis); err == nil {
		return &analysis, nil
	}

	// Look for JSON block delimiters.
	start := -1
	end := -1
	for i, ch := range text {
		if ch == '{' && start == -1 {
			start = i
		}
		if ch == '}' {
			end = i + 1
		}
	}

	if start >= 0 && end > start {
		jsonBlock := text[start:end]
		if err := json.Unmarshal([]byte(jsonBlock), &analysis); err == nil {
			return &analysis, nil
		}
	}

	return nil, fmt.Errorf("no valid JSON found in response")
}

// loadSystemPrompt returns the embedded RCA system prompt.
func loadSystemPrompt() (string, error) {
	if embeddedPrompt == "" {
		return "", fmt.Errorf("embedded prompt is empty")
	}
	return embeddedPrompt, nil
}

const defaultSystemPrompt = `You are a build failure root cause analysis expert. Analyze the provided GitHub Actions workflow failure data and return a JSON response with the following structure:
{
  "category": "CodeChange|Infrastructure|DependencyIssue|TestFailure|Configuration|Unknown",
  "rootCauseSummary": "One-line summary of the root cause",
  "rootCauseDetails": "Detailed explanation of why the workflow failed and what caused it",
  "evidence": ["List of evidence points supporting the analysis"],
  "nextSteps": ["Recommended actions to fix the issue"],
  "confidence": "high|medium|low"
}

Focus on accuracy. Cross-reference all available data sources. Provide actionable next steps.`
