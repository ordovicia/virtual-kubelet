package sim

import (
	"encoding/json"
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type simSpec []simSpecPhase

type simSpecPhase struct {
	Seconds int32  `json:"seconds"`
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	GPU     int32  `json:"nvidia.com/gpu"`
}

func parseSimSpec(pod *v1.Pod) (simSpec, error) {
	simSpecJSON, ok := pod.ObjectMeta.Annotations["simSpec"]
	if !ok {
		return nil, fmt.Errorf("simSpec not defined")
	}

	simSpec := []simSpecPhase{}
	err := json.Unmarshal([](byte)(simSpecJSON), &simSpec)
	if err != nil {
		return nil, err
	}

	for _, phase := range simSpec {
		if _, err = resource.ParseQuantity(phase.CPU); err != nil {
			return nil, fmt.Errorf("Invalid CPU value %q", phase.CPU)
		}
		if _, err = resource.ParseQuantity(phase.Memory); err != nil {
			return nil, fmt.Errorf("Invalid memory value %q", phase.Memory)
		}
	}

	return simSpec, nil
}
