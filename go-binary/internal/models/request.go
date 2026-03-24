package models

// AnalysisRequest represents the input parameters for a build failure analysis.
type AnalysisRequest struct {
	AnalysisID        string `json:"analysisId"`
	Owner             string `json:"owner"`
	Repo              string `json:"repo"`
	RunID             int64  `json:"runId"`
	RunNumber         int    `json:"runNumber"`
	Workflow          string `json:"workflow"`
	Job               string `json:"job"`
	Ref               string `json:"ref"`
	SHA               string `json:"sha"`
	Actor             string `json:"actor"`
	EventName         string `json:"eventName"`
	ServerUrl         string `json:"serverUrl"`
	ApiUrl            string `json:"apiUrl"`
	PullRequestNumber int    `json:"pullRequestNumber"`
	GithubToken       string `json:"githubToken"`

	AWS        AWSConfig        `json:"aws"`
	Kubernetes KubernetesConfig `json:"kubernetes"`
	JFrog      JFrogConfig      `json:"jfrog"`
	Jira       JiraConfig       `json:"jira"`
	Email      EmailConfig      `json:"email"`

	TeamMappings  string `json:"teamMappings"`
	DevopsManager string `json:"devopsManager"`

	CreateIssue      bool `json:"createIssue"`
	CommentOnPr      bool `json:"commentOnPr"`
	CreateJiraTicket bool `json:"createJiraTicket"`
	SendEmail        bool `json:"sendEmail"`

	// Software category selection
	Categories SoftwareCategories `json:"categories"`

	// BitBucket configuration (when repoSoftware is "bitbucket")
	BitBucket BitBucketConfig `json:"bitbucket"`

	// Docker configuration (when clusterType is "docker")
	Docker DockerConfig `json:"docker"`

	// Nexus configuration (when artifactManager is "nexus")
	Nexus NexusConfig `json:"nexus"`
}

// SoftwareCategories allows users to select which software stack to analyze.
type SoftwareCategories struct {
	RepoSoftware    string `json:"repoSoftware"`
	ClusterType     string `json:"clusterType"`
	ArtifactManager string `json:"artifactManager"`
}

// BitBucketConfig holds BitBucket server connection details.
type BitBucketConfig struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// DockerConfig holds Docker daemon connection details.
type DockerConfig struct {
	Host      string `json:"host"`
	TlsCert   string `json:"tlsCert"`
	TlsKey    string `json:"tlsKey"`
	TlsCaCert string `json:"tlsCaCert"`
}

// NexusConfig holds Sonatype Nexus connection details.
type NexusConfig struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// AWSConfig holds Claude AI provider configuration.
// Supports "bedrock" (default), "direct" (Anthropic API), or "max" (Claude Max/Teams).
type AWSConfig struct {
	Provider         string `json:"provider"`         // "bedrock" (default), "direct", "max"
	Region           string `json:"region"`           // AWS region for Bedrock
	ModelId          string `json:"modelId"`          // Model ID (Bedrock or Anthropic format)
	AccessKeyId      string `json:"accessKeyId"`      // AWS access key for Bedrock
	SecretAccessKey  string `json:"secretAccessKey"`  // AWS secret key for Bedrock
	SessionToken     string `json:"sessionToken"`     // AWS session token for Bedrock
	VpcEndpoint      string `json:"vpcEndpoint"`      // VPC endpoint for Bedrock
	AnthropicApiKey  string `json:"anthropicApiKey"`  // API key for direct/max provider
	AnthropicBaseUrl string `json:"anthropicBaseUrl"` // Base URL override (for max/proxy)
}

// KubernetesConfig holds Kubernetes cluster access configuration.
type KubernetesConfig struct {
	ApiUrl    string `json:"apiUrl"`
	Token     string `json:"token"`
	Namespace string `json:"namespace"`
}

// JFrogConfig holds JFrog Artifactory access configuration.
type JFrogConfig struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	ApiKey   string `json:"apiKey"`
}

// JiraConfig holds Jira integration configuration.
type JiraConfig struct {
	Url            string `json:"url"`
	Username       string `json:"username"`
	ApiToken       string `json:"apiToken"`
	Project        string `json:"project"`
	EpicKey        string `json:"epicKey"`
	DevopsAssignee string `json:"devopsAssignee"`
}

// EmailConfig holds SMTP email notification configuration.
type EmailConfig struct {
	SmtpHost    string `json:"smtpHost"`
	SmtpPort    int    `json:"smtpPort"`
	EnableSsl   bool   `json:"enableSsl"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}
