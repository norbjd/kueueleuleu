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
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func ConvertPod(pod corev1.Pod) (corev1.Pod, error) {
	var (
		kueueleuleuPod corev1.Pod
		err            error
	)

	if errCopy := deepCopy(pod, &kueueleuleuPod); errCopy != nil {
		return kueueleuleuPod, fmt.Errorf("can't convert pod: %w", errCopy)
	}

	kueueleuleuPod.ObjectMeta = convertObjectMeta(kueueleuleuPod.ObjectMeta)
	kueueleuleuPod.Spec, err = convertPodSpec(kueueleuleuPod.Spec)

	return kueueleuleuPod, err
}

func ConvertJob(job batchv1.Job) (batchv1.Job, error) {
	var (
		kueueleuleuJob batchv1.Job
		err            error
	)

	if errCopy := deepCopy(job, &kueueleuleuJob); errCopy != nil {
		return kueueleuleuJob, fmt.Errorf("can't convert job: %w", errCopy)
	}

	kueueleuleuJob.ObjectMeta = convertObjectMeta(kueueleuleuJob.ObjectMeta)
	kueueleuleuJob.Spec.Template.ObjectMeta = convertObjectMeta(kueueleuleuJob.Spec.Template.ObjectMeta)
	kueueleuleuJob.Spec.Template.Spec, err = convertPodSpec(kueueleuleuJob.Spec.Template.Spec)

	return kueueleuleuJob, err
}

func ConvertCronJob(cronjob batchv1.CronJob) (batchv1.CronJob, error) {
	var (
		kueueleuleuCronjob batchv1.CronJob
		err                error
	)

	if errCopy := deepCopy(cronjob, &kueueleuleuCronjob); errCopy != nil {
		return kueueleuleuCronjob, fmt.Errorf("can't convert cronjob: %w", errCopy)
	}

	kueueleuleuCronjob.ObjectMeta = convertObjectMeta(kueueleuleuCronjob.ObjectMeta)
	kueueleuleuCronjob.Spec.JobTemplate.ObjectMeta = convertObjectMeta(kueueleuleuCronjob.Spec.JobTemplate.ObjectMeta)
	kueueleuleuCronjob.Spec.JobTemplate.Spec.Template.ObjectMeta = convertObjectMeta(
		kueueleuleuCronjob.Spec.JobTemplate.Spec.Template.ObjectMeta)
	kueueleuleuCronjob.Spec.JobTemplate.Spec.Template.Spec, err = convertPodSpec(
		kueueleuleuCronjob.Spec.JobTemplate.Spec.Template.Spec)

	return kueueleuleuCronjob, err
}

var errInternal = errors.New("internal error")

// deepCopy - copies src to dist
// it might be slow, but we can always improve later if required.
func deepCopy(src, dist any) error {
	buf := bytes.Buffer{}

	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return fmt.Errorf("%w: can't deep copy: %s", errInternal, err.Error())
	}

	if err := gob.NewDecoder(&buf).Decode(dist); err != nil {
		return fmt.Errorf("%w: can't deep copy: %s", errInternal, err.Error())
	}

	return nil
}
