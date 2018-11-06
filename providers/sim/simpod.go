package sim

import (
	"sync"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// podMap stores a map associating pod "key" with *v1.Pod.
// It wraps sync.Map for type-safety.
type podMap struct {
	pods sync.Map
}

// simPod represents a pod in the node with attained time
type simPod struct {
	pod            *v1.Pod
	startTime      metav1.Time
	isOverCapacity bool
	spec           simSpec
}

func (m *podMap) load(key string) (simPod, bool) {
	pod, ok := m.pods.Load(key)
	if !ok {
		return simPod{}, false
	}
	return pod.(simPod), true
}

func (m *podMap) store(key string, pod simPod) {
	m.pods.Store(key, pod)
}

func (m *podMap) delete(key string) {
	m.pods.Delete(key)
}

// listPods returns an array of pods
func (m *podMap) listPods() []*v1.Pod {
	pods := []*v1.Pod{}
	m.pods.Range(func(_, pod interface{}) bool {
		pods = append(pods, pod.(simPod).pod)
		return true
	})
	return pods
}

// foreach applies a function to each pair of key and pod
func (m *podMap) foreach(f func(string, simPod) bool) {
	g := func(key, pod interface{}) bool {
		return f(key.(string), pod.(simPod))
	}
	m.pods.Range(g)
}
