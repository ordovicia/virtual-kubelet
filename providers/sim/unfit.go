package sim

import "k8s.io/api/core/v1"

type resourceRequest struct {
	milliCPU int64
	memory   int64
	gpu      int64
}

func getResourceRequest(pod *v1.Pod) resourceRequest {
	result := resourceRequest{}

	for _, container := range pod.Spec.Containers {
		limits := container.Resources.Limits
		result.milliCPU += limits.Cpu().MilliValue()
		result.memory += limits.Memory().Value()
		if gpu, ok := limits["nvidia.com/gpu"]; ok {
			result.gpu += gpu.Value()
		}
	}

	return result
}

func getUnfitPods(pods []*v1.Pod, capacity v1.ResourceList) []*v1.Pod {
	capacityMilliCPU := capacity.Cpu().MilliValue()
	capacityMemory := capacity.Memory().Value()
	capacityPods := capacity.Pods().Value()

	totalReqMilliCPU := int64(0)
	totalReqMemory := int64(0)

	unfit := []*v1.Pod{}

	for i, pod := range pods {
		if i >= int(capacityPods) {
			unfit = append(unfit, pod)
			continue
		}

		podRequest := getResourceRequest(pod)
		fitsCPU := totalReqMilliCPU+podRequest.milliCPU <= capacityMilliCPU
		fitsMemory := totalReqMemory+podRequest.memory <= capacityMemory

		if !fitsCPU || !fitsMemory {
			unfit = append(unfit, pod)
			continue
		} else {
			totalReqMilliCPU += podRequest.milliCPU
			totalReqMemory += podRequest.memory
		}
	}

	return unfit
}

type podsByCreationTime []*v1.Pod

func (s podsByCreationTime) Len() int {
	return len(s)
}

func (s podsByCreationTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s podsByCreationTime) Less(i, j int) bool {
	return s[i].CreationTimestamp.Before(&s[j].CreationTimestamp)
}
