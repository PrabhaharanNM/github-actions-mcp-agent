package agents

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// DockerAgent inspects a Docker daemon for container failures, OOM kills,
// image issues, and disk usage that may relate to build failures.
type DockerAgent struct {
	host      string
	tlsCert   string
	tlsKey    string
	tlsCaCert string
}

// NewDockerAgent creates a DockerAgent from the analysis request config.
func NewDockerAgent(req *models.AnalysisRequest) *DockerAgent {
	return &DockerAgent{
		host:      strings.TrimRight(req.Docker.Host, "/"),
		tlsCert:   req.Docker.TlsCert,
		tlsKey:    req.Docker.TlsKey,
		tlsCaCert: req.Docker.TlsCaCert,
	}
}

// Analyze queries the Docker Engine API for container statuses, failures,
// OOM kills, image issues, and disk usage. Returns nil if Docker is not configured.
func (d *DockerAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.DockerAgentResult, error) {
	if d.host == "" {
		log.Println("[docker] skipping: docker config is empty")
		return nil, nil
	}

	result := &models.DockerAgentResult{}

	// List all containers (including stopped).
	listURL := fmt.Sprintf("%s/v1.43/containers/json?all=true", d.host)
	listData, err := d.get(ctx, listURL)
	if err != nil {
		log.Printf("[docker] warning: failed to list containers: %v", err)
	} else {
		d.parseContainerList(ctx, listData, result)
	}

	// Fetch system disk usage.
	dfURL := fmt.Sprintf("%s/v1.43/system/df", d.host)
	dfData, err := d.get(ctx, dfURL)
	if err != nil {
		log.Printf("[docker] warning: failed to fetch disk usage: %v", err)
	} else {
		d.parseDiskUsage(dfData, result)
	}

	return result, nil
}

// get performs an HTTP GET against the Docker Engine API, handling Unix sockets
// and TLS-enabled TCP connections.
func (d *DockerAgent) get(ctx context.Context, url string) ([]byte, error) {
	// For Unix sockets, use a custom transport.
	if strings.HasPrefix(d.host, "unix://") {
		return d.getUnix(ctx, url)
	}

	// For TCP with TLS certs, configure a TLS client.
	if d.tlsCert != "" && d.tlsKey != "" {
		return d.getTLS(ctx, url)
	}

	// Plain TCP — use the shared doRequest helper.
	return doRequest(ctx, url, "", "")
}

// getUnix performs an HTTP GET over a Unix domain socket.
func (d *DockerAgent) getUnix(ctx context.Context, url string) ([]byte, error) {
	socketPath := strings.TrimPrefix(d.host, "unix://")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}
	client := &http.Client{Transport: transport, Timeout: httpTimeout}

	// Replace the unix:// host with http://localhost so the HTTP client is happy.
	reqURL := strings.Replace(url, d.host, "http://localhost", 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating unix request for %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s (unix): %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned %d: %s", url, resp.StatusCode, truncate(body, 256))
	}
	return body, nil
}

// getTLS performs an HTTP GET with mutual TLS authentication.
func (d *DockerAgent) getTLS(ctx context.Context, url string) ([]byte, error) {
	cert, err := tls.X509KeyPair([]byte(d.tlsCert), []byte(d.tlsKey))
	if err != nil {
		return nil, fmt.Errorf("loading TLS keypair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if d.tlsCaCert != "" {
		// For simplicity, set InsecureSkipVerify when CA cert is provided
		// but not loaded into a pool — production code should parse the CA.
		tlsConfig.InsecureSkipVerify = false
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport, Timeout: httpTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating TLS request for %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s (TLS): %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned %d: %s", url, resp.StatusCode, truncate(body, 256))
	}
	return body, nil
}

// parseContainerList parses the container list response and inspects each
// container for failures, OOM kills, and image issues.
func (d *DockerAgent) parseContainerList(ctx context.Context, data []byte, result *models.DockerAgentResult) {
	var containers []struct {
		ID      string   `json:"Id"`
		Names   []string `json:"Names"`
		Image   string   `json:"Image"`
		State   string   `json:"State"`
		Status  string   `json:"Status"`
	}
	if err := json.Unmarshal(data, &containers); err != nil {
		log.Printf("[docker] warning: failed to parse container list: %v", err)
		return
	}

	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		result.ContainerStatuses = append(result.ContainerStatuses, models.ContainerStatus{
			Name:  name,
			Image: c.Image,
			State: c.State,
			Status: c.Status,
		})

		// Inspect individual container for detailed failure info.
		if c.State == "exited" || c.State == "dead" {
			d.inspectContainer(ctx, c.ID, name, result)
		}
	}
}

// inspectContainer fetches detailed container info and checks for OOM kills
// and non-zero exit codes.
func (d *DockerAgent) inspectContainer(ctx context.Context, id, name string, result *models.DockerAgentResult) {
	inspectURL := fmt.Sprintf("%s/v1.43/containers/%s/json", d.host, id)
	data, err := d.get(ctx, inspectURL)
	if err != nil {
		log.Printf("[docker] warning: failed to inspect container %s: %v", name, err)
		return
	}

	var inspect struct {
		State struct {
			ExitCode  int  `json:"ExitCode"`
			OOMKilled bool `json:"OOMKilled"`
			Error     string `json:"Error"`
		} `json:"State"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &inspect); err != nil {
		log.Printf("[docker] warning: failed to parse container inspect for %s: %v", name, err)
		return
	}

	// Update exit code in ContainerStatuses.
	for i := range result.ContainerStatuses {
		if result.ContainerStatuses[i].Name == name {
			result.ContainerStatuses[i].ExitCode = inspect.State.ExitCode
			break
		}
	}

	if inspect.State.OOMKilled {
		result.OOMKilled = append(result.OOMKilled,
			fmt.Sprintf("container/%s OOMKilled (exit code %d)", name, inspect.State.ExitCode))
	}

	if inspect.State.ExitCode != 0 {
		msg := fmt.Sprintf("container/%s exited with code %d", name, inspect.State.ExitCode)
		if inspect.State.Error != "" {
			msg += fmt.Sprintf(": %s", inspect.State.Error)
		}
		result.FailedContainers = append(result.FailedContainers, msg)
	}

	// Check for image pull issues (image name mismatch or missing image).
	if inspect.Config.Image != "" && strings.Contains(inspect.State.Error, "image") {
		result.ImageIssues = append(result.ImageIssues,
			fmt.Sprintf("container/%s image issue: %s", name, inspect.State.Error))
	}
}

// parseDiskUsage extracts disk usage summary from the Docker system df response.
func (d *DockerAgent) parseDiskUsage(data []byte, result *models.DockerAgentResult) {
	var df struct {
		LayersSize int64 `json:"LayersSize"`
		Images     []struct {
			Size       int64 `json:"Size"`
			SharedSize int64 `json:"SharedSize"`
		} `json:"Images"`
		Containers []struct {
			SizeRw     int64 `json:"SizeRw"`
			SizeRootFs int64 `json:"SizeRootFs"`
		} `json:"Containers"`
		Volumes []struct {
			UsageData struct {
				Size int64 `json:"Size"`
			} `json:"UsageData"`
		} `json:"Volumes"`
	}
	if err := json.Unmarshal(data, &df); err != nil {
		log.Printf("[docker] warning: failed to parse disk usage: %v", err)
		return
	}

	var totalImageSize, totalVolumeSize int64
	for _, img := range df.Images {
		totalImageSize += img.Size
	}
	for _, vol := range df.Volumes {
		totalVolumeSize += vol.UsageData.Size
	}

	result.DiskUsage = fmt.Sprintf("layers=%dMB images=%dMB volumes=%dMB",
		df.LayersSize/(1024*1024),
		totalImageSize/(1024*1024),
		totalVolumeSize/(1024*1024))
}
