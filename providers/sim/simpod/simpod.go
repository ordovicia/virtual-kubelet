package simpod

import (
	"sync"

	"k8s.io/api/core/v1"
)

// PodMap stores a map associating pod "key" with *v1.Pod
type PodMap struct {
	pods sync.Map
}

type RunningPod struct {
	Pod             *v1.Pod
	AttainedSeconds int32
}

func New() *PodMap {
	return &PodMap{}
}

func (m *PodMap) Load(key string) (RunningPod, bool) {
	pod, ok := m.pods.Load(key)
	if !ok {
		return RunningPod{}, false
	}
	return pod.(RunningPod), true
}

func (m *PodMap) Store(key string, pod RunningPod) {
	m.pods.Store(key, pod)
}

func (m *PodMap) Delete(key string) {
	m.pods.Delete(key)
}

func (m *PodMap) Range(f func(string, RunningPod) bool) {
	m.Range(f)
}
