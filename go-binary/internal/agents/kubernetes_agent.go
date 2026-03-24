package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/github-actions-mcp-agent/go-binary/internal/models"
)

// KubernetesAgent inspects a Kubernetes cluster for infrastructure-related
// build failure evidence such as OOM kills and node pressure.
type KubernetesAgent struct {
	apiUrl    string
	token     string
	namespace string
}

// NewKubernetesAgent creates a KubernetesAgent from the analysis request config.
func NewKubernetesAgent(req *models.AnalysisRequest) *KubernetesAgent {
	return &KubernetesAgent{
		apiUrl:    strings.TrimRight(req.Kubernetes.ApiUrl, "/"),
		token:     req.Kubernetes.Token,
		namespace: req.Kubernetes.Namespace,
	}
}

// Analyze fetches pod statuses and node conditions from the Kubernetes API.
// Returns nil result if Kubernetes is not configured.
func (k *KubernetesAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.KubernetesAgentResult, error) {
	if k.apiUrl == "" || k.token == "" {
		log.Println("[k8s] skipping: kubernetes config is empty")
		return nil, nil
	}

	ns := k.namespace
	if ns == "" {
		ns = "default"
	}

	result := &models.KubernetesAgentResult{}

	// Fetch pods in the namespace.
	podsURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods", k.apiUrl, ns)
	podsData, err := k.get(ctx, podsURL)
	if err != nil {
		log.Printf("[k8s] warning: failed to fetch pods: %v", err)
	} else {
		k.parsePods(podsData, result)
	}

	// Fetch namespace events to find additional error signals.
	eventsURL := fmt.Sprintf("%s/api/v1/namespaces/%s/events", k.apiUrl, ns)
	eventsData, err := k.get(ctx, eventsURL)
	if err != nil {
		log.Printf("[k8s] warning: failed to fetch events: %v", err)
	} else {
		k.parseEvents(eventsData, result)
	}

	// Check node conditions for memory/disk pressure.
	nodesURL := fmt.Sprintf("%s/api/v1/nodes", k.apiUrl)
	nodesData, err := k.get(ctx, nodesURL)
	if err != nil {
		log.Printf("[k8s] warning: failed to fetch nodes: %v", err)
	} else {
		k.parseNodePressure(nodesData, result)
	}

	return result, nil
}

// get performs a bearer-token-authenticated GET against the Kubernetes API.
func (k *KubernetesAgent) get(ctx context.Context, url string) ([]byte, error) {
	return doRequest(ctx, url, "Authorization", "Bearer "+k.token)
}

// parsePods extracts pod statuses and detects OOM kills from the pods response.
func (k *KubernetesAgent) parsePods(data []byte, result *models.KubernetesAgentResult) {
	var podList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				Reason            string `json:"reason"`
				ContainerStatuses []struct {
					Name         string `json:"name"`
					RestartCount int    `json:"restartCount"`
					LastState    struct {
						Terminated *struct {
							Reason   string `json:"reason"`
							ExitCode int    `json:"exitCode"`
						} `json:"terminated"`
					} `json:"lastState"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &podList); err != nil {
		log.Printf("[k8s] warning: failed to parse pods: %v", err)
		return
	}

	for _, pod := range podList.Items {
		totalRestarts := 0
		for _, cs := range pod.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
			if cs.LastState.Terminated != nil &&
				strings.EqualFold(cs.LastState.Terminated.Reason, "OOMKilled") {
				result.OOMKills = append(result.OOMKills,
					fmt.Sprintf("pod/%s container/%s OOMKilled (exit code %d)",
						pod.Metadata.Name, cs.Name, cs.LastState.Terminated.ExitCode))
			}
		}
		result.PodStatuses = append(result.PodStatuses, models.PodStatus{
			Name:         pod.Metadata.Name,
			Phase:        pod.Status.Phase,
			Reason:       pod.Status.Reason,
			RestartCount: totalRestarts,
		})
	}
}

// parseEvents extracts warning events from the namespace events response.
func (k *KubernetesAgent) parseEvents(data []byte, result *models.KubernetesAgentResult) {
	var eventList struct {
		Items []struct {
			Type    string `json:"type"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
			InvolvedObject struct {
				Name string `json:"name"`
				Kind string `json:"kind"`
			} `json:"involvedObject"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &eventList); err != nil {
		log.Printf("[k8s] warning: failed to parse events: %v", err)
		return
	}

	for _, e := range eventList.Items {
		if strings.EqualFold(e.Type, "Warning") {
			result.Events = append(result.Events,
				fmt.Sprintf("%s/%s: %s - %s", e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Reason, e.Message))
		}
	}
}

// parseNodePressure checks node conditions for MemoryPressure, DiskPressure, or PIDPressure.
func (k *KubernetesAgent) parseNodePressure(data []byte, result *models.KubernetesAgentResult) {
	var nodeList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &nodeList); err != nil {
		log.Printf("[k8s] warning: failed to parse nodes: %v", err)
		return
	}

	pressureTypes := map[string]bool{
		"MemoryPressure": true,
		"DiskPressure":   true,
		"PIDPressure":    true,
	}
	for _, node := range nodeList.Items {
		for _, cond := range node.Status.Conditions {
			if pressureTypes[cond.Type] && strings.EqualFold(cond.Status, "True") {
				result.NodePressure = true
				result.Events = append(result.Events,
					fmt.Sprintf("node/%s has %s", node.Metadata.Name, cond.Type))
			}
		}
	}
}
