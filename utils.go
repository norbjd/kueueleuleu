// This file is part of kueueleuleu (https://github.com/norbjd/kueueleuleu).
//
// Copyright (C) 2023 norbjd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3 of the License.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package kueueleuleu

import (
	"errors"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrNotAKueueleuleuPod               = errors.New("not a kueueleuleu pod")
	ErrPodIsSucceeded                   = errors.New("pod is succeeded")
	ErrPodIsFailed                      = errors.New("pod is failed")
	ErrPodIsNotRunning                  = errors.New("pod is not running")
	ErrSentinelAllContainersAreFinished = errors.New("all containers are finished")
)

func IsKueueleuleu(meta metav1.ObjectMeta) bool {
	annotations := meta.GetAnnotations()

	return annotations != nil &&
		annotations[kueueleuleuAnnotationKey] == kueueleuleuAnnotationValue
}

func GetRunningContainerName(pod corev1.Pod) (string, error) {
	if !IsKueueleuleu(pod.ObjectMeta) {
		return "", ErrNotAKueueleuleuPod
	}

	err := checkPodPhaseIsValid(pod.Status.Phase)
	if err != nil {
		return "", err
	}

	allContainerStatusesSorted := getContainerStatusesSorted(pod)

	for _, containerStatus := range allContainerStatusesSorted {
		if containerStatus.Name == prepareInitContainerName {
			continue
		}

		containerName := containerStatus.Name

		if containerStatus.State.Terminated == nil || containerStatus.State.Terminated.FinishedAt.IsZero() {
			return containerName, nil
		}
	}

	return "", ErrSentinelAllContainersAreFinished
}

func checkPodPhaseIsValid(podPhase corev1.PodPhase) error {
	// PodUnknown is the only missing case, but it is deprecated and not set since 2015
	// see https://github.com/kubernetes/kubernetes/blob/v1.29.0/pkg/apis/core/types.go#L2721-L2724
	//nolint: exhaustive
	switch podPhase {
	case corev1.PodRunning, corev1.PodPending:
		return nil
	case corev1.PodSucceeded:
		return ErrPodIsSucceeded
	case corev1.PodFailed:
		return ErrPodIsFailed
	default:
		return fmt.Errorf("%w: got phase %s", ErrPodIsNotRunning, podPhase)
	}
}

// getContainerStatusesSorted - sort container statuses according to the containers orders
// by default, containers statuses are sorted by name (alphabetically)
// and not by the order containers are declared in the pod spec.
func getContainerStatusesSorted(pod corev1.Pod) []corev1.ContainerStatus {
	allContainers := pod.Spec.InitContainers
	allContainers = append(allContainers, pod.Spec.Containers...)
	allContainerNamesAndOrder := make(map[string]int)

	for i, container := range allContainers {
		allContainerNamesAndOrder[container.Name] = i
	}

	// unfortunately, container statuses are not sorted...
	sort.Slice(pod.Status.InitContainerStatuses, func(i, j int) bool {
		return allContainerNamesAndOrder[pod.Status.InitContainerStatuses[i].Name] <
			allContainerNamesAndOrder[pod.Status.InitContainerStatuses[j].Name]
	})
	sort.Slice(pod.Status.ContainerStatuses, func(i, j int) bool {
		return allContainerNamesAndOrder[pod.Status.ContainerStatuses[i].Name] <
			allContainerNamesAndOrder[pod.Status.ContainerStatuses[j].Name]
	})

	allContainerStatusesSorted := pod.Status.InitContainerStatuses
	allContainerStatusesSorted = append(allContainerStatusesSorted, pod.Status.ContainerStatuses...)

	return allContainerStatusesSorted
}
