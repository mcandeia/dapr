/*
Copyright 2022 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package patcher

import (
	"fmt"

	"github.com/dapr/dapr/pkg/injector/annotations"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// GetEnvPatchOperations adds new environment variables only if they do not exist.
// It does not override existing values for those variables if they have been defined already.
func GetEnvPatchOperations(envs []corev1.EnvVar, addEnv []corev1.EnvVar, containerIdx int) []PatchOperation {
	path := fmt.Sprintf("%s/%d/env", PatchPathContainers, containerIdx)
	if len(envs) == 0 {
		// If there are no environment variables defined in the container, we initialize a slice of environment vars.
		return []PatchOperation{
			{
				Op:    "add",
				Path:  path,
				Value: addEnv,
			},
		}
	}

	// If there are existing env vars, then we are adding to an existing slice of env vars.
	path += "/-"

	patchOps := make([]PatchOperation, len(addEnv))
	n := 0
	for _, env := range addEnv {
		isConflict := false
		for _, actual := range envs {
			if actual.Name == env.Name {
				// Add only env vars that do not conflict with existing user defined/injected env vars.
				isConflict = true
				break
			}
		}

		if isConflict {
			continue
		}

		patchOps[n] = PatchOperation{
			Op:    "add",
			Path:  path,
			Value: env,
		}
		n++
	}
	return patchOps[:n]
}

// GetVolumeMountPatchOperations gets the patch operations for volume mounts
func GetVolumeMountPatchOperations(volumeMounts []corev1.VolumeMount, addMounts []corev1.VolumeMount, containerIdx int) []PatchOperation {
	path := fmt.Sprintf("%s/%d/volumeMounts", PatchPathContainers, containerIdx)
	if len(volumeMounts) == 0 {
		// If there are no volume mounts defined in the container, we initialize a slice of volume mounts.
		return []PatchOperation{
			{
				Op:    "add",
				Path:  path,
				Value: addMounts,
			},
		}
	}

	// If there are existing volume mounts, then we are adding to an existing slice of volume mounts.
	path += "/-"

	patchOps := make([]PatchOperation, len(addMounts))
	n := 0
	for _, addMount := range addMounts {
		isConflict := false
		for _, mount := range volumeMounts {
			// conflict cases
			if addMount.Name == mount.Name || addMount.MountPath == mount.MountPath {
				isConflict = true
				break
			}
		}

		if isConflict {
			continue
		}

		patchOps[n] = PatchOperation{
			Op:    "add",
			Path:  path,
			Value: addMount,
		}
		n++
	}

	return patchOps[:n]
}

// GetResourceRequirementsFunc returns a function that receives an annotation map and returns the resource limits based on the provided annotations.
func GetResourceRequirementsFunc(targetContainer, keyCPULimit, keyMemoryLimit, keyCPURequest, keyMemoryRequest string) func(an annotations.Map) (*corev1.ResourceRequirements, error) {
	return func(an annotations.Map) (*corev1.ResourceRequirements, error) {
		return getResourceRequirements(an, targetContainer, keyCPULimit, keyMemoryLimit, keyCPURequest, keyMemoryRequest)
	}
}

// getResourceRequirements get the container resource requirements based on annotated cpu and memory limits/requests.
func getResourceRequirements(an annotations.Map, targetContainer, keyCPULimit, keyMemoryLimit, keyCPURequest, keyMemoryRequest string) (*corev1.ResourceRequirements, error) {
	r := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}
	cpuLimit, ok := an[keyCPULimit]
	if ok {
		list, err := appendQuantityToResourceList(cpuLimit, corev1.ResourceCPU, r.Limits)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing %s cpu limit", targetContainer)
		}
		r.Limits = *list
	}
	memLimit, ok := an[keyMemoryLimit]
	if ok {
		list, err := appendQuantityToResourceList(memLimit, corev1.ResourceMemory, r.Limits)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing %s memory limit", targetContainer)
		}
		r.Limits = *list
	}
	cpuRequest, ok := an[keyCPURequest]
	if ok {
		list, err := appendQuantityToResourceList(cpuRequest, corev1.ResourceCPU, r.Requests)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing %s cpu request", targetContainer)
		}
		r.Requests = *list
	}
	memRequest, ok := an[keyMemoryRequest]
	if ok {
		list, err := appendQuantityToResourceList(memRequest, corev1.ResourceMemory, r.Requests)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing %s memory request", targetContainer)
		}
		r.Requests = *list
	}

	if len(r.Limits) > 0 || len(r.Requests) > 0 {
		return &r, nil
	}
	return nil, nil
}

// appendQuantityToResourceList append the given quantity to the current container resource list.
func appendQuantityToResourceList(quantity string, resourceName corev1.ResourceName, resourceList corev1.ResourceList) (*corev1.ResourceList, error) {
	q, err := resource.ParseQuantity(quantity)
	if err != nil {
		return nil, err
	}
	resourceList[resourceName] = q
	return &resourceList, nil
}
