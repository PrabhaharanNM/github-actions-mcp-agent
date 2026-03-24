package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/orchestrator"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			errResult := models.AnalysisResult{
				Status:       "error",
				ErrorMessage: fmt.Sprintf("panic: %v", r),
			}
			out, _ := json.Marshal(errResult)
			fmt.Fprintln(os.Stderr, string(out))
			os.Exit(1)
		}
	}()

	if len(os.Args) < 2 || os.Args[1] != "analyze" {
		fmt.Fprintln(os.Stderr, "usage: mcp-agent analyze --request '<json>'")
		os.Exit(1)
	}

	var requestJSON string
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--request" && i+1 < len(args) {
			requestJSON = args[i+1]
			break
		}
	}

	if requestJSON == "" {
		fmt.Fprintln(os.Stderr, "error: --request flag is required")
		os.Exit(1)
	}

	var req models.AnalysisRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		errResult := models.AnalysisResult{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("failed to parse request JSON: %v", err),
		}
		out, _ := json.Marshal(errResult)
		fmt.Println(string(out))
		os.Exit(1)
	}

	if req.AnalysisID == "" {
		req.AnalysisID = uuid.New().String()
	}

	ctx := context.Background()
	result, err := orchestrator.Analyze(ctx, &req)
	if err != nil {
		errResult := models.AnalysisResult{
			Status:       "error",
			ErrorMessage: err.Error(),
		}
		out, _ := json.Marshal(errResult)
		fmt.Println(string(out))
		os.Exit(1)
	}

	out, err := json.Marshal(result)
	if err != nil {
		errResult := models.AnalysisResult{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("failed to marshal result: %v", err),
		}
		errOut, _ := json.Marshal(errResult)
		fmt.Println(string(errOut))
		os.Exit(1)
	}

	fmt.Println(string(out))
}
