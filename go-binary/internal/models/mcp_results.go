package models

// McpResults aggregates results from all MCP agent sources.
type McpResults struct {
	GithubResult     *GithubAgentResult     `json:"github_result"`
	BitBucketResult  *BitBucketAgentResult  `json:"bitbucket_result"`
	KubernetesResult *KubernetesAgentResult `json:"kubernetes_result"`
	DockerResult     *DockerAgentResult     `json:"docker_result"`
	JFrogResult      *JFrogAgentResult      `json:"jfrog_result"`
	NexusResult      *NexusAgentResult      `json:"nexus_result"`
}

// GithubAgentResult holds data gathered from GitHub APIs.
type GithubAgentResult struct {
	WorkflowRun   string       `json:"workflow_run"`
	Jobs          []JobInfo    `json:"jobs"`
	RecentCommits []CommitInfo `json:"recent_commits"`
	Codeowners    string       `json:"codeowners"`
	ChangedFiles  []string     `json:"changed_files"`
	PrTitle       string       `json:"pr_title"`
	PrBody        string       `json:"pr_body"`
}

// JobInfo represents a workflow job and its steps.
type JobInfo struct {
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Conclusion string     `json:"conclusion"`
	Steps      []StepInfo `json:"steps"`
}

// StepInfo represents a single step within a job.
type StepInfo struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// CommitInfo represents a recent commit.
type CommitInfo struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Message string `json:"message"`
	Date    string `json:"date"`
}

// KubernetesAgentResult holds data gathered from Kubernetes cluster inspection.
type KubernetesAgentResult struct {
	PodStatuses  []PodStatus `json:"pod_statuses"`
	OOMKills     []string    `json:"oom_kills"`
	Events       []string    `json:"events"`
	NodePressure bool        `json:"node_pressure"`
}

// PodStatus represents the status of a Kubernetes pod.
type PodStatus struct {
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Reason       string `json:"reason"`
	RestartCount int    `json:"restart_count"`
}

// JFrogAgentResult holds data gathered from JFrog Artifactory.
type JFrogAgentResult struct {
	ArtifactsAvailable bool     `json:"artifacts_available"`
	MissingArtifacts   []string `json:"missing_artifacts"`
	RepositoryStatus   string   `json:"repository_status"`
}

// BitBucketAgentResult holds data gathered from BitBucket APIs.
type BitBucketAgentResult struct {
	CodeOwners    string       `json:"code_owners"`
	RecentCommits []CommitInfo `json:"recent_commits"`
	ChangedFiles  []string     `json:"changed_files"`
}

// DockerAgentResult holds data gathered from the Docker Engine API.
type DockerAgentResult struct {
	ContainerStatuses []ContainerStatus `json:"container_statuses"`
	FailedContainers  []string          `json:"failed_containers"`
	OOMKilled         []string          `json:"oom_killed"`
	ImageIssues       []string          `json:"image_issues"`
	DiskUsage         string            `json:"disk_usage"`
}

// ContainerStatus describes the status of a Docker container.
type ContainerStatus struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	State    string `json:"state"`
	Status   string `json:"status"`
	ExitCode int    `json:"exit_code"`
}

// NexusAgentResult holds data gathered from Sonatype Nexus.
type NexusAgentResult struct {
	ArtifactsAvailable bool     `json:"artifacts_available"`
	MissingArtifacts   []string `json:"missing_artifacts"`
	RepositoryStatus   string   `json:"repository_status"`
}

// Correlation represents the cross-agent correlation of root cause findings.
type Correlation struct {
	RootCauseType         string   `json:"root_cause_type"`
	ResponsibleRepository string   `json:"responsible_repository"`
	ResponsibleTeam       string   `json:"responsible_team"`
	IsInfrastructure      bool     `json:"is_infrastructure"`
	Evidence              []string `json:"evidence"`
}

// TeamManager represents the manager responsible for a team.
type TeamManager struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	JiraUsername string `json:"jira_username"`
}
