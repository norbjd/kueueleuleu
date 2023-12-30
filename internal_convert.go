package kueueleuleu

import (
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prepareInitContainerName = "kueueleuleu-prepare"
	tektonEntrypointImage    = "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint" +
		"@sha256:40abc3a78b558f251e890085972ed25fe7ad428f47998bc9c9c18f564dc03c32"

	kueueleuleuAnnotationKey   = "norbjd.github.io/kueueleuleu"
	kueueleuleuAnnotationValue = "true"
)

var ErrContainerDoesNotHaveACommand = errors.New("container does not have a command, but we expect one")

func convertObjectMeta(objectMeta metav1.ObjectMeta) metav1.ObjectMeta {
	if objectMeta.Annotations == nil {
		objectMeta.Annotations = make(map[string]string)
	}

	objectMeta.Annotations[kueueleuleuAnnotationKey] = kueueleuleuAnnotationValue

	return objectMeta
}

func checkPodSpecIsValid(podSpec corev1.PodSpec) error {
	var err error

	for _, container := range podSpec.Containers {
		if len(container.Command) == 0 {
			err = errors.Join(err,
				fmt.Errorf("%w (container %s)", ErrContainerDoesNotHaveACommand, container.Name),
			)
		}
	}

	return err
}

//nolint:funlen
func convertPodSpec(podSpec corev1.PodSpec) (corev1.PodSpec, error) {
	errInvalidPodSpec := checkPodSpecIsValid(podSpec)
	if errInvalidPodSpec != nil {
		return corev1.PodSpec{}, fmt.Errorf("pod spec is invalid: %w", errInvalidPodSpec)
	}

	kueueleuleuPodSpec := podSpec

	initContainer := corev1.Container{
		Name:    prepareInitContainerName,
		Image:   tektonEntrypointImage,
		Command: strings.Split("/ko-app/entrypoint init /ko-app/entrypoint /tekton/bin/entrypoint", " "),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "tekton-internal-bin",
				MountPath: "/tekton/bin",
			},
			{
				Name:      "tekton-internal-steps",
				MountPath: "/tekton/steps",
			},
		},
	}

	// prepend our own init container
	kueueleuleuPodSpec.InitContainers = append([]corev1.Container{initContainer}, podSpec.InitContainers...)

	newVolumes := kueueleuleuPodSpec.Volumes
	newVolumes = append(newVolumes,
		[]corev1.Volume{
			{
				// this is only used in the init container because /tekton/steps must exist, and is not useful otherwise
				Name: "tekton-internal-steps",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "tekton-internal-bin",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}...,
	)

	for i := range podSpec.Containers {
		volume := corev1.Volume{
			Name:         fmt.Sprintf("tekton-internal-run-%d", i),
			VolumeSource: corev1.VolumeSource{},
		}

		newVolumes = append(newVolumes, volume)
	}

	kueueleuleuPodSpec.Volumes = newVolumes

	for index, container := range podSpec.Containers {
		newVolumeMounts := container.VolumeMounts
		newVolumeMounts = append(newVolumeMounts, []corev1.VolumeMount{
			{
				Name:      "tekton-internal-bin",
				MountPath: "/tekton/bin",
				ReadOnly:  true,
			},
		}...)

		for otherContainerIndex := range podSpec.Containers {
			volumeMount := corev1.VolumeMount{
				Name:      fmt.Sprintf("tekton-internal-run-%d", otherContainerIndex),
				MountPath: fmt.Sprintf("/tekton/run/%d", otherContainerIndex),
			}
			if index != otherContainerIndex {
				volumeMount.ReadOnly = true
			}

			newVolumeMounts = append(newVolumeMounts, volumeMount)
		}

		container.VolumeMounts = newVolumeMounts

		newArgs := make([]string, 0)

		if index > 0 {
			newArgs = []string{
				"-wait_file",
				fmt.Sprintf("/tekton/run/%d/out", index-1),
			}
		}

		newArgs = append(newArgs, []string{
			"-post_file",
			fmt.Sprintf("/tekton/run/%d/out", index),
			"-step_metadata_dir",
			fmt.Sprintf("/tekton/run/%d/status", index),
			"-entrypoint",
		}...)

		newArgs = append(newArgs, container.Command[0])
		newArgs = append(newArgs, "--")

		if len(container.Command) > 1 {
			newArgs = append(newArgs, container.Command[1:]...)
		}

		newArgs = append(newArgs, container.Args...)

		container.Args = newArgs
		container.Command = []string{"/tekton/bin/entrypoint"}

		kueueleuleuPodSpec.Containers[index] = container
	}

	return kueueleuleuPodSpec, nil
}
