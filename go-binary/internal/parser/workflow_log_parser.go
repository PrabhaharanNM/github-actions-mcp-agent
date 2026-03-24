package parser

import (
	"regexp"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

var (
	// errorAnnotationRe matches GitHub Actions error annotations: ##[error]...
	errorAnnotationRe = regexp.MustCompile(`##\[error\](.+)`)
	// exitCodeRe matches process exit code errors used to identify the failed job.
	exitCodeRe = regexp.MustCompile(`##\[error\]Process completed with exit code \d+`)
	// stepGroupRe matches step group headers: ##[group]Run ...
	stepGroupRe = regexp.MustCompile(`##\[group\]Run\s+(.+)`)
	// stepRunRe matches bare "Run ..." step headers.
	stepRunRe = regexp.MustCompile(`^Run\s+(.+)`)
	// genericErrorRe matches lines containing common error keywords.
	genericErrorRe = regexp.MustCompile(`(?i)(?:Error:|FAILED|error:)\s*(.+)`)
	// testFailureRe matches common test failure patterns across languages.
	testFailureRe = regexp.MustCompile(`(?i)(?:FAIL|FAILED|FAILURE)[:\s]+(\S+)`)
	// goTestFailRe matches Go-style test failure output.
	goTestFailRe = regexp.MustCompile(`--- FAIL:\s+(\S+)`)
	// junitFailRe matches JUnit-style test failure output.
	junitFailRe = regexp.MustCompile(`(?i)Tests run:.*Failures:\s*([1-9]\d*)`)
	// jobNameRe matches GitHub Actions job name headers in logs.
	jobNameRe = regexp.MustCompile(`^(?:##\[group\])?(?:Run |Set up job|Complete job)\s*(.*)`)
)

// Parse extracts structured build context from raw GitHub Actions workflow logs.
func Parse(logs string) *models.BuildContext {
	ctx := &models.BuildContext{
		ConsoleLog: logs,
	}

	lines := strings.Split(logs, "\n")

	var (
		currentStep string
		failedStep  string
		failedJob   string
		errors      []string
		failedTests []string
		allJobs     []string
		jobSet      = make(map[string]bool)
	)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Track step headers to know which step failed.
		if m := stepGroupRe.FindStringSubmatch(line); len(m) > 1 {
			currentStep = strings.TrimSpace(m[1])
		} else if m := stepRunRe.FindStringSubmatch(line); len(m) > 1 {
			currentStep = strings.TrimSpace(m[1])
		}

		// Collect job names from structured headers.
		if m := jobNameRe.FindStringSubmatch(line); len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if name != "" && !jobSet[name] {
				jobSet[name] = true
				allJobs = append(allJobs, name)
			}
		}

		// Detect the failed job/step from exit-code error annotations.
		if exitCodeRe.MatchString(line) {
			if failedStep == "" && currentStep != "" {
				failedStep = currentStep
			}
		}

		// Collect error annotations.
		if m := errorAnnotationRe.FindStringSubmatch(line); len(m) > 1 {
			msg := strings.TrimSpace(m[1])
			if msg != "" && !isDuplicate(errors, msg) {
				errors = append(errors, msg)
			}
		}

		// Collect generic error lines (Error:, FAILED, error:).
		if m := genericErrorRe.FindStringSubmatch(line); len(m) > 1 {
			// Avoid duplicating ##[error] lines already captured above.
			if !strings.HasPrefix(line, "##[error]") {
				msg := strings.TrimSpace(m[0])
				if !isDuplicate(errors, msg) {
					errors = append(errors, msg)
				}
			}
		}

		// Collect failed test names.
		if m := goTestFailRe.FindStringSubmatch(line); len(m) > 1 {
			failedTests = appendUnique(failedTests, m[1])
		} else if m := testFailureRe.FindStringSubmatch(line); len(m) > 1 {
			failedTests = appendUnique(failedTests, m[1])
		}
		if junitFailRe.MatchString(line) {
			failedTests = appendUnique(failedTests, line)
		}
	}

	// If we found a failed step but no explicit failed job, derive it from the step.
	if failedStep != "" && failedJob == "" {
		failedJob = deriveJobFromStep(failedStep, allJobs)
	}

	ctx.FailedStep = failedStep
	ctx.FailedJob = failedJob
	ctx.ErrorMessages = errors
	ctx.FailedTests = failedTests
	ctx.AllJobs = allJobs
	ctx.SuspectedRepository = deriveSuspectedRepository(failedJob, failedStep, errors)

	return ctx
}

// deriveJobFromStep tries to find the matching job name for a given step.
func deriveJobFromStep(step string, jobs []string) string {
	stepLower := strings.ToLower(step)
	for _, j := range jobs {
		if strings.Contains(stepLower, strings.ToLower(j)) {
			return j
		}
	}
	// If no match, return the first job as a best guess.
	if len(jobs) > 0 {
		return jobs[0]
	}
	return ""
}

// deriveSuspectedRepository scans job/step names and error messages for repository references.
func deriveSuspectedRepository(job, step string, errors []string) string {
	repoPattern := regexp.MustCompile(`([a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+)`)

	// Check step name first (most specific).
	if m := repoPattern.FindString(step); m != "" && looksLikeRepo(m) {
		return m
	}
	// Check job name.
	if m := repoPattern.FindString(job); m != "" && looksLikeRepo(m) {
		return m
	}
	// Check error messages.
	for _, e := range errors {
		if m := repoPattern.FindString(e); m != "" && looksLikeRepo(m) {
			return m
		}
	}
	return ""
}

// looksLikeRepo returns true if the string looks like an org/repo slug rather than a file path.
func looksLikeRepo(s string) bool {
	// Exclude common file-path patterns.
	if strings.Contains(s, ".go") || strings.Contains(s, ".js") ||
		strings.Contains(s, ".ts") || strings.Contains(s, ".py") ||
		strings.HasPrefix(s, "./") || strings.HasPrefix(s, "/") {
		return false
	}
	parts := strings.Split(s, "/")
	return len(parts) == 2 && len(parts[0]) > 0 && len(parts[1]) > 0
}

func isDuplicate(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func appendUnique(slice []string, s string) []string {
	if !isDuplicate(slice, s) {
		return append(slice, s)
	}
	return slice
}
