import * as core from '@actions/core';
import * as exec from '@actions/exec';
import * as github from '@actions/github';
import * as io from '@actions/io';
import * as tc from '@actions/tool-cache';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

interface AnalysisResult {
  status: string;
  category: string;
  rootCauseSummary: string;
  rootCauseDetails: string;
  responsibleTeam: string;
  teamEmail: string;
  htmlReport: string;
  jiraTicketKey: string;
  jiraTicketUrl: string;
  githubIssueUrl: string;
  evidence: string[];
  nextSteps: string[];
  errorMessage: string;
  analysisTimeMs: number;
}

async function run(): Promise<void> {
  try {
    const context = github.context;

    // Only run on workflow failure
    const conclusion = process.env.WORKFLOW_CONCLUSION || 'failure';
    if (conclusion === 'success') {
      core.info('Workflow succeeded - skipping analysis.');
      return;
    }

    core.info('MCP Agent: Starting AI-powered failure analysis...');

    // Generate analysis ID
    const analysisId = `gha-${context.runId}-${Date.now()}`;
    core.setOutput('analysis-id', analysisId);

    // Build request JSON from inputs
    const request = buildRequest(analysisId, context);

    // Get the Go binary path
    const binaryPath = await getBinaryPath();

    // Execute Go binary
    let stdout = '';
    let stderr = '';
    const exitCode = await exec.exec(binaryPath, [
      'analyze',
      '--request', JSON.stringify(request),
    ], {
      listeners: {
        stdout: (data: Buffer) => { stdout += data.toString(); },
        stderr: (data: Buffer) => { stderr += data.toString(); },
      },
      ignoreReturnCode: true,
    });

    if (stderr) {
      core.debug(`Go binary stderr: ${stderr}`);
    }

    // Parse result
    let result: AnalysisResult;
    try {
      result = JSON.parse(stdout);
    } catch {
      core.warning(`Failed to parse analysis output: ${stdout.substring(0, 500)}`);
      return;
    }

    // Set outputs
    core.setOutput('category', result.category);
    core.setOutput('root-cause-summary', result.rootCauseSummary);
    core.setOutput('responsible-team', result.responsibleTeam);
    core.setOutput('jira-ticket-key', result.jiraTicketKey);
    core.setOutput('github-issue-url', result.githubIssueUrl);

    // Save HTML report
    if (result.htmlReport) {
      const reportDir = path.join(os.tmpdir(), 'mcp-reports');
      fs.mkdirSync(reportDir, { recursive: true });
      const reportPath = path.join(reportDir, `${analysisId}.html`);
      fs.writeFileSync(reportPath, result.htmlReport);
      core.setOutput('html-report-path', reportPath);
    }

    // Write job summary
    await writeJobSummary(result, analysisId);

    if (result.status === 'completed') {
      core.info(`Analysis complete: ${result.category} - ${result.rootCauseSummary}`);
    } else {
      core.warning(`Analysis finished with status: ${result.status} - ${result.errorMessage}`);
    }

  } catch (error) {
    if (error instanceof Error) {
      core.warning(`MCP Agent failed: ${error.message}`);
    }
    // Never fail the workflow because of analysis errors
  }
}

function buildRequest(analysisId: string, context: typeof github.context): Record<string, unknown> {
  return {
    analysisId,
    // GitHub context
    owner: context.repo.owner,
    repo: context.repo.repo,
    runId: context.runId,
    runNumber: context.runNumber,
    workflow: context.workflow,
    job: context.job,
    ref: context.ref,
    sha: context.sha,
    actor: context.actor,
    eventName: context.eventName,
    serverUrl: context.serverUrl,
    apiUrl: context.apiUrl,
    pullRequestNumber: context.payload.pull_request?.number || 0,

    // GitHub token
    githubToken: core.getInput('github-token'),

    // AWS Bedrock
    aws: {
      region: core.getInput('aws-region'),
      modelId: core.getInput('aws-model-id'),
      accessKeyId: core.getInput('aws-access-key-id'),
      secretAccessKey: core.getInput('aws-secret-access-key'),
      sessionToken: core.getInput('aws-session-token'),
      vpcEndpoint: core.getInput('aws-vpc-endpoint'),
    },

    // Kubernetes
    kubernetes: {
      apiUrl: core.getInput('k8s-api-url'),
      token: core.getInput('k8s-token'),
      namespace: core.getInput('k8s-namespace'),
    },

    // JFrog
    jfrog: {
      url: core.getInput('jfrog-url'),
      username: core.getInput('jfrog-username'),
      apiKey: core.getInput('jfrog-api-key'),
    },

    // Jira
    jira: {
      url: core.getInput('jira-url'),
      username: core.getInput('jira-username'),
      apiToken: core.getInput('jira-api-token'),
      project: core.getInput('jira-project'),
      epicKey: core.getInput('jira-epic-key'),
      devopsAssignee: core.getInput('jira-devops-assignee'),
    },

    // Email
    email: {
      smtpHost: core.getInput('smtp-host'),
      smtpPort: parseInt(core.getInput('smtp-port')) || 587,
      enableSsl: core.getInput('smtp-ssl') === 'true',
      fromAddress: core.getInput('smtp-from-address'),
      fromName: core.getInput('smtp-from-name'),
      username: core.getInput('smtp-username'),
      password: core.getInput('smtp-password'),
    },

    // Team config
    teamMappings: core.getInput('team-mappings'),
    devopsManager: core.getInput('devops-manager'),

    // Feature flags
    createIssue: core.getInput('create-issue') === 'true',
    commentOnPr: core.getInput('comment-on-pr') === 'true',
    createJiraTicket: core.getInput('create-jira-ticket') === 'true',
    sendEmail: core.getInput('send-email') === 'true',

    // Software categories
    categories: {
      repoSoftware: core.getInput('repo-software'),
      clusterType: core.getInput('cluster-type'),
      artifactManager: core.getInput('artifact-manager'),
    },

    // BitBucket (when repo-software is "bitbucket")
    bitbucket: {
      url: core.getInput('bitbucket-url'),
      username: core.getInput('bitbucket-username'),
      password: core.getInput('bitbucket-password'),
    },

    // Docker (when cluster-type is "docker")
    docker: {
      host: core.getInput('docker-host'),
      tlsCert: core.getInput('docker-tls-cert'),
      tlsKey: core.getInput('docker-tls-key'),
      tlsCaCert: core.getInput('docker-tls-ca-cert'),
    },

    // Nexus (when artifact-manager is "nexus")
    nexus: {
      url: core.getInput('nexus-url'),
      username: core.getInput('nexus-username'),
      password: core.getInput('nexus-password'),
    },
  };
}

async function getBinaryPath(): Promise<string> {
  const platform = os.platform();
  const arch = os.arch();

  let binaryName: string;
  if (platform === 'linux' && arch === 'x64') {
    binaryName = 'mcp-agent-linux-amd64';
  } else if (platform === 'darwin' && arch === 'x64') {
    binaryName = 'mcp-agent-darwin-amd64';
  } else if (platform === 'darwin' && arch === 'arm64') {
    binaryName = 'mcp-agent-darwin-arm64';
  } else if (platform === 'win32' && arch === 'x64') {
    binaryName = 'mcp-agent-windows-amd64.exe';
  } else {
    throw new Error(`Unsupported platform: ${platform}/${arch}`);
  }

  // Binary is bundled in the action's binaries directory
  const binaryPath = path.join(__dirname, '..', 'binaries', binaryName);

  if (!fs.existsSync(binaryPath)) {
    throw new Error(`Go binary not found at ${binaryPath}`);
  }

  // Make executable on Unix
  if (platform !== 'win32') {
    await exec.exec('chmod', ['+x', binaryPath]);
  }

  return binaryPath;
}

async function writeJobSummary(result: AnalysisResult, analysisId: string): Promise<void> {
  const statusEmoji = result.status === 'completed' ? ':white_check_mark:' : ':x:';

  let summary = `## ${statusEmoji} MCP Build Failure Analysis\n\n`;
  summary += `**Analysis ID:** \`${analysisId}\`\n\n`;

  if (result.status === 'completed') {
    summary += `| Field | Value |\n|-------|-------|\n`;
    summary += `| **Category** | ${result.category} |\n`;
    summary += `| **Responsible Team** | ${result.responsibleTeam} |\n`;
    summary += `| **Analysis Time** | ${result.analysisTimeMs}ms |\n\n`;

    summary += `### Root Cause\n${result.rootCauseSummary}\n\n`;

    if (result.rootCauseDetails) {
      summary += `<details><summary>Detailed Analysis</summary>\n\n${result.rootCauseDetails}\n\n</details>\n\n`;
    }

    if (result.evidence && result.evidence.length > 0) {
      summary += `### Evidence\n`;
      for (const e of result.evidence) {
        summary += `- ${e}\n`;
      }
      summary += '\n';
    }

    if (result.nextSteps && result.nextSteps.length > 0) {
      summary += `### Next Steps\n`;
      for (let i = 0; i < result.nextSteps.length; i++) {
        summary += `${i + 1}. ${result.nextSteps[i]}\n`;
      }
      summary += '\n';
    }

    if (result.jiraTicketKey) {
      summary += `**Jira:** [${result.jiraTicketKey}](${result.jiraTicketUrl})\n`;
    }
    if (result.githubIssueUrl) {
      summary += `**GitHub Issue:** ${result.githubIssueUrl}\n`;
    }
  } else {
    summary += `:warning: Analysis failed: ${result.errorMessage}\n`;
  }

  await core.summary.addRaw(summary).write();
}

run();
