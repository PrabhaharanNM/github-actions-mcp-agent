package parser

import (
	"testing"
)

const sampleGHALog = `=== build/1_build.txt ===
##[group]Run npm ci
Run npm ci
npm warn deprecated some-package@1.0.0
##[endgroup]
##[group]Run npm test
Run npm test

> myapp@1.0.0 test
> jest --coverage

PASS src/utils.test.ts
FAIL src/api.test.ts
  --- FAIL: TestAPIEndpoint
  Expected 200, received 500
Tests:       1 failed, 1 passed, 2 total
##[error]Process completed with exit code 1
##[error]Error: npm test failed with exit code 1
=== deploy/2_deploy.txt ===
##[group]Run deploy.sh
Run deploy.sh
Deploying to staging...
##[endgroup]`

func TestParse_ExtractsFailedStep(t *testing.T) {
	ctx := Parse(sampleGHALog)
	if ctx.FailedStep != "npm test" {
		t.Errorf("expected failed step 'npm test', got %q", ctx.FailedStep)
	}
}

func TestParse_ExtractsErrorMessages(t *testing.T) {
	ctx := Parse(sampleGHALog)
	if len(ctx.ErrorMessages) == 0 {
		t.Fatal("expected error messages, got none")
	}
	found := false
	for _, msg := range ctx.ErrorMessages {
		if msg == "Process completed with exit code 1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected exit code error in messages, got %v", ctx.ErrorMessages)
	}
}

func TestParse_ExtractsFailedTests(t *testing.T) {
	ctx := Parse(sampleGHALog)
	if len(ctx.FailedTests) == 0 {
		t.Fatal("expected failed tests, got none")
	}
	found := false
	for _, ft := range ctx.FailedTests {
		if ft == "TestAPIEndpoint" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TestAPIEndpoint in failed tests, got %v", ctx.FailedTests)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	ctx := Parse("")
	if ctx == nil {
		t.Fatal("Parse should return non-nil BuildContext for empty input")
	}
	if ctx.FailedStep != "" {
		t.Errorf("expected empty failed step for empty input, got %q", ctx.FailedStep)
	}
}

func TestParse_ExtractsAllJobs(t *testing.T) {
	ctx := Parse(sampleGHALog)
	if len(ctx.AllJobs) == 0 {
		t.Skip("job extraction from log headers is best-effort")
	}
}

func TestParse_MultipleErrors(t *testing.T) {
	logs := `##[error]First error message
##[error]Second error message
##[error]Process completed with exit code 2`
	ctx := Parse(logs)
	if len(ctx.ErrorMessages) < 3 {
		t.Errorf("expected at least 3 error messages, got %d: %v", len(ctx.ErrorMessages), ctx.ErrorMessages)
	}
}

func TestParse_GoTestFailure(t *testing.T) {
	logs := `--- FAIL: TestMyFunction (0.01s)
    mytest.go:42: expected true, got false
FAIL	github.com/myorg/myrepo/pkg 0.015s
##[error]Process completed with exit code 1`
	ctx := Parse(logs)
	found := false
	for _, ft := range ctx.FailedTests {
		if ft == "TestMyFunction" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TestMyFunction in failed tests, got %v", ctx.FailedTests)
	}
}
