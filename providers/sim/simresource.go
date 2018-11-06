package sim

import "k8s.io/api/core/v1"

type simResource struct {
	milliCPU int64
	memory   int64
	gpu      int64
}

func (r simResource) add(rhs simResource) simResource {
	return simResource{
		milliCPU: r.milliCPU + rhs.milliCPU,
		memory:   r.memory + rhs.memory,
		gpu:      r.gpu + rhs.gpu,
	}
}

func (r simResource) sub(rhs simResource) simResource {
	return simResource{
		milliCPU: r.milliCPU - rhs.milliCPU,
		memory:   r.memory - rhs.memory,
		gpu:      r.gpu - rhs.gpu,
	}
}

func getResourceReq(pod *v1.Pod) simResource {
	result := simResource{}

	for _, container := range pod.Spec.Containers {
		req := container.Resources.Requests
		result.milliCPU += req.Cpu().MilliValue()
		result.memory += req.Memory().Value()
		if gpu, ok := req["nvidia.com/gpu"]; ok {
			result.gpu += gpu.Value()
		}
	}

	return result
}

func isOverCapacity(req simResource, capacity v1.ResourceList) bool {
	isOver := req.milliCPU > capacity.Cpu().MilliValue() ||
		req.memory > capacity.Memory().Value()
	if gpu, ok := capacity["nvidia.com/gpu"]; ok {
		isOver = isOver || req.gpu > gpu.Value()
	}
	return isOver
}
