package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	// Provider configuration defaults
	defaultCPUCapacity    = "16"
	defaultMemoryCapacity = "64Gi"
	defaultGPUCapacity    = "2"
	defaultPodCapacity    = "20"

	// Operating system representation
	operatingSystemSimulated = "Simulated"
)

// Provider implements the virtual-kubelet provider interface and stores pods in memory.
// The type of operating system is represented as "Simulated"
type Provider struct {
	nodeName           string
	internalIP         string
	daemonEndpointPort int32
	config             Config
	pods               *podMap
	totalResourceUsage simResource
}

// Config contains a simulated virtual-kubelet's configurable parameters.
type Config struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	GPU    string `json:"nvidia.com/gpu,omitempty"`
	Pods   string `json:"pods,omitempty"`
}

// NewSimProvider creates a new SimProvider
func NewSimProvider(providerConfig, nodeName string, internalIP string, daemonEndpointPort int32) (*Provider, error) {
	config, err := loadConfig(providerConfig, nodeName)
	if err != nil {
		return nil, err
	}

	provider := Provider{
		nodeName:           nodeName,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
		pods:               &podMap{},
		config:             config,
	}

	go updateNode(&provider, 1*time.Second)
	return &provider, nil
}

// loadConfig loads the given json configuration file.
// If the file does not exist, falls back on the default config.
func loadConfig(providerConfig, nodeName string) (Config, error) {
	var config Config

	if _, err := os.Stat(providerConfig); os.IsNotExist(err) {
		config.CPU = defaultCPUCapacity
		config.Memory = defaultMemoryCapacity
		config.GPU = defaultGPUCapacity
		config.Pods = defaultPodCapacity
		return config, nil
	}

	data, err := ioutil.ReadFile(providerConfig)
	if err != nil {
		return config, err
	}

	configMap := map[string]Config{}
	err = json.Unmarshal(data, &configMap)
	if err != nil {
		return config, err
	}

	if _, exist := configMap[nodeName]; exist {
		config = configMap[nodeName]
		if config.CPU == "" {
			config.CPU = defaultCPUCapacity
		}
		if config.Memory == "" {
			config.Memory = defaultMemoryCapacity
		}
		if config.GPU == "" {
			config.GPU = defaultGPUCapacity
		}
		if config.Pods == "" {
			config.Pods = defaultPodCapacity
		}
	}

	if _, err = resource.ParseQuantity(config.CPU); err != nil {
		return config, fmt.Errorf("Invalid CPU value %q", config.CPU)
	}
	if _, err = resource.ParseQuantity(config.Memory); err != nil {
		return config, fmt.Errorf("Invalid memory value %q", config.Memory)
	}
	if _, err = resource.ParseQuantity(config.Pods); err != nil {
		return config, fmt.Errorf("Invalid pods value %q", config.Pods)
	}

	return config, nil
}

func updateNode(p *Provider, interval time.Duration) {
	tick := time.Tick(interval)
	for range tick {
		now := time.Now()
		p.totalResourceUsage = simResource{}

		p.pods.foreach(func(key string, pod simPod) bool {
			passedSeconds := int32(now.Sub(pod.startTime).Seconds())
			if pod.isTerminated(passedSeconds) {
				// pod.status = simPodTerminated
			} else {
				p.totalResourceUsage = p.totalResourceUsage.add(pod.resourceUsage(passedSeconds))
			}

			return true
		})
	}
}

// CreatePodccepts a Pod definition and stores it in memory.
func (p *Provider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	log.Printf("receive CreatePod %q\n", pod.Name)

	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	simSpec, err := parseSimSpec(pod)
	if err != nil {
		return err
	}

	now := time.Now()
	simPod := simPod{pod: pod, startTime: now, spec: simSpec}
	newTotalReq := p.totalResourceReq(now).add(getResourceReq(pod))
	cap := p.Capacity(ctx)
	if isOverCapacity(newTotalReq, cap) || p.runningPodsNum(now) >= cap.Pods().Value() {
		simPod.status = simPodOverCapacity
	} else {
		simPod.status = simPodOk
	}
	p.pods.store(key, simPod)

	return nil
}

func (p *Provider) totalResourceReq(now time.Time) simResource {
	resourceReq := simResource{}
	p.pods.foreach(func(_ string, pod simPod) bool {
		passedSeconds := int32(now.Sub(pod.startTime).Seconds())
		if pod.status == simPodOk && !pod.isTerminated(passedSeconds) {
			resourceReq = resourceReq.add(getResourceReq(pod.pod))
		}
		return true
	})
	return resourceReq
}

func (p *Provider) runningPodsNum(now time.Time) int64 {
	podsNum := int64(0)
	p.pods.foreach(func(_ string, pod simPod) bool {
		passedSeconds := int32(now.Sub(pod.startTime).Seconds())
		if pod.status == simPodOk && !pod.isTerminated(passedSeconds) {
			podsNum++
		}
		return true
	})
	return podsNum
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *Provider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	log.Printf("receive UpdatePod %q\n", pod.Name)

	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	simPod, ok := p.pods.load(key)
	if !ok {
		return fmt.Errorf("pod %q does not exist", key)
	}

	simPod.pod = pod
	p.pods.store(key, simPod)

	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *Provider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	log.Printf("receive DeletePod %q\n", pod.Name)

	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	p.pods.delete(key)

	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *Provider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	log.Printf("receive GetPod %q\n", name)

	pod, err := p.getSimPod(namespace, name)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, nil
	}

	return pod.pod, nil
}

func (p *Provider) getSimPod(namespace, name string) (*simPod, error) {
	key, err := buildKeyFromNames(namespace, name)
	if err != nil {
		return nil, err
	}

	pod, ok := p.pods.load(key)
	if !ok {
		return nil, nil
	}

	return &pod, nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
// TODO: Implementation
func (p *Provider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	log.Printf("receive GetContainerLogs %q\n", podName)
	return "", nil
}

// GetPodFullName gets full pod name as defined in the provider context
// TODO: Implementation
func (p *Provider) GetPodFullName(namespace string, pod string) string {
	log.Printf("receive GetPodFullName %q, %q\n", namespace, pod)
	return ""
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *Provider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus returns the status of a pod by name that is "running".
// It returns nil if a pod by that name is not found.
func (p *Provider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	log.Printf("receive GetPodStatus %q\n", name)

	pod, err := p.getSimPod(namespace, name)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, nil
	}

	var status v1.PodStatus
	switch pod.status {
	case simPodOverCapacity:
		status = v1.PodStatus{
			Phase:   v1.PodFailed,
			Reason:  "CapacityExceeded",
			Message: "Pod cannot be started due to exceeded capacity",
		}
	case simPodOk:
		now := time.Now()
		passedSeconds := int32(now.Sub(pod.startTime).Seconds())

		startTime := metav1.NewTime(pod.startTime)
		if pod.isTerminated(passedSeconds) {
			finishTime := metav1.NewTime(pod.startTime.Add(time.Duration(pod.totalSeconds()) * time.Second))
			status = buildPodStatus(pod, v1.PodSucceeded, startTime,
				v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						ExitCode:   0,
						Reason:     "Succeeded", // TODO
						Message:    "All containers in the pod have voluntarily terminated",
						StartedAt:  startTime,
						FinishedAt: finishTime,
					}})
		} else {
			status = buildPodStatus(pod, v1.PodRunning, startTime,
				v1.ContainerState{
					Running: &v1.ContainerStateRunning{
						StartedAt: startTime,
					}})
		}
	}

	return &status, nil
}

func buildPodStatus(pod *simPod, phase v1.PodPhase, startTime metav1.Time, containerState v1.ContainerState) v1.PodStatus {
	status := v1.PodStatus{
		Phase:     phase,
		HostIP:    "1.2.3.4",
		PodIP:     "5.6.7.8",
		StartTime: &startTime,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodInitialized,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodScheduled,
				Status: v1.ConditionTrue,
			},
		},
	}

	for _, container := range pod.pod.Spec.Containers {
		status.ContainerStatuses = append(status.ContainerStatuses, v1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        true,
			RestartCount: 0,
			State:        containerState,
		})
	}

	return status
}

// GetPods returns a list of all pods known to be "running".
func (p *Provider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	log.Printf("receive GetPods\n")
	return p.pods.listPods(), nil
}

// Capacity returns a resource list containing the capacity limits.
func (p *Provider) Capacity(ctx context.Context) v1.ResourceList {
	return v1.ResourceList{
		"cpu":            resource.MustParse(p.config.CPU),
		"memory":         resource.MustParse(p.config.Memory),
		"nvidia.com/gpu": resource.MustParse(p.config.GPU),
		"pods":           resource.MustParse(p.config.Pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *Provider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	// TODO: Make this configurable
	return []v1.NodeCondition{
		{
			Type:               "Ready",
			Status:             v1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "kubelet is ready.",
		},
		{
			Type:               "OutOfDisk",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientDisk",
			Message:            "kubelet has sufficient disk space available",
		},
		{
			Type:               "MemoryPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               "DiskPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               "NetworkUnavailable",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "RouteCreated",
			Message:            "RouteController created a route",
		},
	}
}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *Provider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *Provider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider, i.e. "Simulated".
func (p *Provider) OperatingSystem() string {
	return operatingSystemSimulated
}

// buildKey is a helper for building the "key" for the providers pod store.
func buildKey(pod *v1.Pod) (string, error) {
	if pod.ObjectMeta.Namespace == "" {
		return "", fmt.Errorf("pod namespace not found")
	}

	if pod.ObjectMeta.Name == "" {
		return "", fmt.Errorf("pod name not found")
	}

	return buildKeyFromNames(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
}

func buildKeyFromNames(namespace string, name string) (string, error) {
	return fmt.Sprintf("%s-%s", namespace, name), nil
}
