package main

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// patchOperation is an operation of a JSON patch, see
// https://tools.ietf.org/html/rfc6902.
type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

const volumeName = "elastic-apm-agent"

var (
	volumeMounts = corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/elastic/apm/agent",
	}
	agentVolume = corev1.Volume{
		Name:         volumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
)

func createPatch(config agentConfig, spec corev1.PodSpec) []patchOperation {
	// Create patch operations
	var patches []patchOperation

	envVariables := generateEnvironmentVariables(config)

	// Add a volume mount to the pod
	patches = append(patches, createVolumePatch(spec.Volumes == nil))

	// Add an init container, that will fetch the agent Docker image and extract the agent jar to the agent volume
	patches = append(patches, createInitContainerPatch(spec.InitContainers == nil))

	// Add agent env variables for each container at the pod, as well as the volume mount
	containers := spec.Containers
	for index, container := range containers {
		patches = append(patches, createVolumeMountsPatch(container.VolumeMounts == nil, index))
		patches = append(patches, createEnvVariablesPatches(envVariables, container.Env == nil, index)...)
	}
	return patches
}

func generateEnvironmentVariables(config agentConfig) []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 1, len(config.environment)+1)
	vars[0] = corev1.EnvVar{
		Name: "ELASTIC_APM_SECRET_TOKEN",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "apm-server-apm-token"},
				Key:                  "secret-token",
			},
		},
	}
	for name, value := range config.environment {
		vars = append(vars, corev1.EnvVar{Name: name, Value: value})
	}
	return vars
}

func createVolumePatch(createArray bool) patchOperation {
	var patch patchOperation
	if createArray {
		patch = patchOperation{
			Op:    "add",
			Path:  "/spec/volumes",
			Value: []corev1.Volume{agentVolume},
		}
	} else {
		patch = patchOperation{
			Op:    "add",
			Path:  "/spec/volumes/-",
			Value: agentVolume,
		}
	}
	return patch
}

func createInitContainerPatch(createArray bool) patchOperation {
	agentInitContainer := corev1.Container{
		Name:         "elastic-java-agent",
		Image:        "docker.elastic.co/observability/apm-agent-java:1.23.0",
		VolumeMounts: []corev1.VolumeMount{volumeMounts},
		Command:      []string{"cp", "-v", "/usr/agent/elastic-apm-agent.jar", "/elastic/apm/agent"},
	}
	var patch patchOperation
	if createArray {
		patch = patchOperation{
			Op:    "add",
			Path:  "/spec/initContainers",
			Value: []corev1.Container{agentInitContainer},
		}
	} else {
		patch = patchOperation{
			Op:    "add",
			Path:  "/spec/initContainers/-",
			Value: agentInitContainer,
		}
	}
	return patch
}

// If the evn variable array does not already exist, this method will return a single patch operation for the addition of the entire list,
// otherwise it would return a list of patches for each env variable
func createEnvVariablesPatches(envVariables []corev1.EnvVar, createArray bool, index int) []patchOperation {
	containerIndex := strconv.Itoa(index)
	envVariablesPath := "/spec/containers/" + containerIndex + "/env"
	var patches []patchOperation
	if createArray {
		patches = []patchOperation{{
			Op:   "add",
			Path: envVariablesPath,
			// TODO: Store this in the config
			Value: envVariables,
		}}
	} else {
		envVariablesPath += "/-"
		patches = []patchOperation{}
		for _, variable := range envVariables {
			patches = append(patches, patchOperation{
				Op:    "add",
				Path:  envVariablesPath,
				Value: variable,
			})
		}
	}
	return patches
}

func createVolumeMountsPatch(createArray bool, index int) patchOperation {
	containerIndex := strconv.Itoa(index)
	volumeMountsPath := "/spec/containers/" + containerIndex + "/volumeMounts"
	var patch patchOperation
	if createArray {
		patch = patchOperation{
			Op:    "add",
			Path:  volumeMountsPath,
			Value: []corev1.VolumeMount{volumeMounts},
		}
	} else {
		patch = patchOperation{
			Op:    "add",
			Path:  volumeMountsPath + "/-",
			Value: volumeMounts,
		}
	}
	return patch
}
