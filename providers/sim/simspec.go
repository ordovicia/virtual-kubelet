package sim

import (
	"encoding/json"
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type simSpec []simSpecPhase

type simSpecPhase struct {
	seconds  int32
	resource simResource
}

func parseSimSpec(pod *v1.Pod) (simSpec, error) {
	type simSpecPhaseJSON struct {
		Seconds int32  `json:"seconds"`
		CPU     string `json:"cpu"`
		Memory  string `json:"memory"`
		GPU     int64  `json:"nvidia.com/gpu,omitempty"`
	}

	simSpecAnnotation, ok := pod.ObjectMeta.Annotations["simSpec"]
	if !ok {
		return nil, fmt.Errorf("simSpec not defined")
	}

	simSpecJSON := []simSpecPhaseJSON{}
	err := json.Unmarshal([](byte)(simSpecAnnotation), &simSpecJSON)
	if err != nil {
		return nil, err
	}

	simSpec := simSpec{}
	for _, phase := range simSpecJSON {
		cpu, err := resource.ParseQuantity(phase.CPU)
		if err != nil {
			return nil, fmt.Errorf("Invalid CPU value %q", phase.CPU)
		}
		milliCPU := cpu.MilliValue()

		mem, err := resource.ParseQuantity(phase.Memory)
		if err != nil {
			return nil, fmt.Errorf("Invalid memory value %q", phase.Memory)
		}
		memory := mem.Value()

		gpu := phase.GPU
		p := simSpecPhase{seconds: phase.Seconds, resource: simResource{milliCPU, memory, gpu}}
		simSpec = append(simSpec, p)
	}

	return simSpec, nil
}
