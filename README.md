# MCP Agent: AI-Powered Build Failure Analysis for GitHub Actions

Automatically analyzes GitHub Actions workflow failures using AI (Claude via AWS Bedrock) with parallel MCP agents gathering data from GitHub, Kubernetes, and JFrog Artifactory.

## Architecture

```
GitHub Actions Runner
  |
  v
TypeScript Action (src/index.ts)    <-- thin wrapper
  |
  v
Go Binary (go-binary/)              <-- heavy processing
  |
  +-- GitHub Agent       (parallel)  --> GitHub API (runs, jobs, commits, PRs)
  +-- Kubernetes Agent   (parallel)  --> K8s API (pods, events, nodes)
  +-- JFrog Agent        (parallel)  --> Artifactory API (health, storage)
  |
  v
Cross-Correlation Engine             --> deterministic priority-based analysis
  |
  v
Claude AI (AWS Bedrock)              --> structured root cause analysis
  |
  v
Outputs: GitHub Issue, PR Comment, Jira Ticket, Email, HTML Report
```

## Quick Start

```yaml
name: Build Failure Analysis
on:
  workflow_run:
    workflows: ["CI"]
    types: [completed]

jobs:
  analyze:
    runs-on: ubuntu-latest
    if: ${{ github.event.workflow_run.conclusion == 'failure' }}
    steps:
      - uses: actions/checkout@v4
      - uses: PrabhaharanNM/github-actions-mcp-agent@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          create-issue: true
          comment-on-pr: true
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `github-token` | No | `${{ github.token }}` | GitHub token for API access |
| `aws-access-key-id` | Yes | | AWS access key for Bedrock |
| `aws-secret-access-key` | Yes | | AWS secret key for Bedrock |
| `aws-region` | No | `us-west-2` | AWS region |
| `aws-model-id` | No | `anthropic.claude-3-5-sonnet-20241022-v2:0` | Claude model ID |
| `k8s-api-url` | No | | Kubernetes API URL |
| `k8s-token` | No | | Kubernetes auth token |
| `jfrog-url` | No | | JFrog Artifactory URL |
| `jfrog-api-key` | No | | JFrog API key |
| `jira-url` | No | | Jira URL for ticket creation |
| `jira-api-token` | No | | Jira API token |
| `jira-project` | No | `E3` | Jira project key |
| `create-issue` | No | `true` | Create GitHub issue |
| `comment-on-pr` | No | `true` | Comment on PR |
| `create-jira-ticket` | No | `false` | Create Jira ticket |
| `send-email` | No | `false` | Send email notification |
| `team-mappings` | No | `{}` | JSON team-to-manager mapping |

## Outputs

| Output | Description |
|--------|-------------|
| `analysis-id` | Unique analysis identifier |
| `category` | Failure category (CodeChange, Infrastructure, DependencyIssue, etc.) |
| `root-cause-summary` | One-line root cause summary |
| `responsible-team` | Assigned team name |
| `jira-ticket-key` | Created Jira ticket key |
| `github-issue-url` | Created GitHub issue URL |
| `html-report-path` | Path to the HTML report artifact |

## Building from Source

```bash
# Build Go binary
cd go-binary
go mod tidy
bash build.sh

# Build TypeScript action
npm install
npm run build
```

## How It Works

1. **Trigger**: Runs when a monitored workflow fails
2. **Log Fetch**: Downloads workflow run logs from GitHub API
3. **Parse**: Extracts failed steps, error messages, test failures from logs
4. **MCP Agents** (parallel):
   - **GitHub Agent**: Fetches jobs, commits, CODEOWNERS, PR details
   - **Kubernetes Agent**: Checks pod statuses, OOM kills, node pressure
   - **JFrog Agent**: Verifies artifact availability, dependency resolution
5. **Cross-Correlation**: Deterministic 5-priority decision tree identifies root cause category
6. **Claude AI**: Deep analysis via AWS Bedrock for detailed explanation and next steps
7. **Team Assignment**: Maps failure to responsible team via JSON configuration
8. **Integrations**: Creates issues, posts PR comments, Jira tickets, sends emails
