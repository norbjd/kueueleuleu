package kueueleuleu_test

import (
	"testing"

	"github.com/norbjd/kueueleuleu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func toPtr[T any](x T) *T {
	return &x
}

var podSpec = corev1.PodSpec{
	Volumes: []corev1.Volume{
		{
			Name: "volume1",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "volume2",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	},
	InitContainers: []corev1.Container{
		{
			Name:       "dummy-init-container",
			Image:      "alpine",
			Command:    []string{"echo"},
			Args:       []string{"toto"},
			WorkingDir: "/tmp",
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "volume1",
					MountPath: "/tmp/volume1",
				},
			},
		},
	},
	Containers: []corev1.Container{
		{
			Name:    "container1",
			Image:   "alpine",
			Command: []string{"touch"},
			Args:    []string{"/tmp/volume2/test.txt"},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "volume2",
					MountPath: "/tmp/volume2",
				},
			},
		},
		{
			Name:       "aaa",
			Image:      "alpine",
			Command:    []string{"ls"},
			Args:       []string{"-al", "/tmp/volume1"},
			WorkingDir: "/",
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "volume1",
					MountPath: "/tmp/volume1",
				},
			},
		},
		{
			Name:    "container3",
			Image:   "alpine",
			Command: []string{"stat"},
			Args:    []string{"/tmp/volume2/test.txt"},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("100M"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "volume2",
					MountPath: "/tmp/volume2",
				},
			},
		},
	},
	RestartPolicy:                "Never",
	AutomountServiceAccountToken: toPtr(false),
}

func Test_ConvertPod(t *testing.T) {
	t.Parallel()

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy",
		},
		Spec: podSpec,
	}

	assert.False(t, kueueleuleu.IsKueueleuleu(pod.ObjectMeta))

	kueueleuleuPod, err := kueueleuleu.ConvertPod(pod)
	require.NoError(t, err)
	assert.True(t, kueueleuleu.IsKueueleuleu(kueueleuleuPod.ObjectMeta))
}

func Test_ConvertJob(t *testing.T) {
	t.Parallel()

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
			BackoffLimit:            toPtr(int32(1)),
			TTLSecondsAfterFinished: toPtr(int32(100)),
		},
	}

	assert.False(t, kueueleuleu.IsKueueleuleu(job.ObjectMeta))

	kueueleuleuJob, err := kueueleuleu.ConvertJob(job)
	require.NoError(t, err)
	assert.True(t, kueueleuleu.IsKueueleuleu(kueueleuleuJob.ObjectMeta))
}

func Test_ConvertCronJob(t *testing.T) {
	t.Parallel()

	cronjob := batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy",
		},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
					BackoffLimit:            toPtr(int32(1)),
					TTLSecondsAfterFinished: toPtr(int32(100)),
				},
			},
			Schedule: "0 0 * * *",
		},
	}

	assert.False(t, kueueleuleu.IsKueueleuleu(cronjob.ObjectMeta))

	kueueleuleuCronJob, err := kueueleuleu.ConvertCronJob(cronjob)
	require.NoError(t, err)
	assert.True(t, kueueleuleu.IsKueueleuleu(kueueleuleuCronJob.ObjectMeta))
}
