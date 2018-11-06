package simpod

import (
	"sync"

	"k8s.io/api/core/v1"
)

// PodMap stores a map associating pod "key" with *v1.Pod.
// It wraps sync.Map for type-safety.
type PodMap struct {
	pods sync.Map
}

// RunningPod represents a pod in the node with attained time
type RunningPod struct {
	Pod             *v1.Pod
	AttainedSeconds int32
}

// New create a new PodMap
func New() *PodMap {
	return &PodMap{}
}

// Load loads a RunningPod by key and whether the pod exists
func (m *PodMap) Load(key string) (RunningPod, bool) {
	pod, ok := m.pods.Load(key)
	if !ok {
		return RunningPod{}, false
	}
	return pod.(RunningPod), true
}

// Store stores a pod with a key
func (m *PodMap) Store(key string, pod RunningPod) {
	m.pods.Store(key, pod)
}

// Delete deletes a pod by key
func (m *PodMap) Delete(key string) {
	m.pods.Delete(key)
}

// ListPods returns an array of pods
func (m *PodMap) ListPods() []*v1.Pod {
	pods := []*v1.Pod{}
	m.pods.Range(func(_, pod interface{}) bool {
		pods = append(pods, pod.(RunningPod).Pod)
		return true
	})
	return pods
}
