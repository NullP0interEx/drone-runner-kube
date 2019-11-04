// Copyright 2019 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package engine

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func toPod(spec *Spec) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        spec.PodSpec.Name,
			Namespace:   spec.PodSpec.Namespace,
			Annotations: spec.PodSpec.Annotations,
			Labels:      spec.PodSpec.Labels,
		},
		Spec: v1.PodSpec{
			ServiceAccountName: spec.PodSpec.ServiceAccountName,
			RestartPolicy:      v1.RestartPolicyNever,
			Volumes:            toVolumes(spec),
			Containers:         toContainers(spec),
			NodeSelector:       spec.PodSpec.NodeSelector,
			Tolerations:        toTolerations(spec),
		},
	}
}

func toTolerations(spec *Spec) []v1.Toleration {
	var tolerations []v1.Toleration
	for _, toleration := range spec.PodSpec.Tolerations {
		tolerations = append(tolerations, v1.Toleration{
			Operator:          v1.TolerationOperator(toleration.Operator),
			Effect:            v1.TaintEffect(toleration.Effect),
			TolerationSeconds: int64ptr(int64(toleration.TolerationSeconds)),
			Value:             toleration.Value,
		})
	}
	return tolerations
}

func toVolumes(spec *Spec) []v1.Volume {
	var volumes []v1.Volume
	for _, v := range spec.Volumes {
		if v.EmptyDir != nil {
			volume := v1.Volume{
				Name: v.EmptyDir.ID,
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			}
			volumes = append(volumes, volume)
		}

		if v.HostPath != nil {
			hostPathType := v1.HostPathDirectoryOrCreate
			volume := v1.Volume{
				Name: v.HostPath.ID,
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: v.HostPath.Path,
						Type: &hostPathType,
					},
				},
			}
			volumes = append(volumes, volume)
		}
	}

	return volumes
}

func toContainers(spec *Spec) []v1.Container {
	var containers []v1.Container

	for _, s := range spec.Steps {
		container := v1.Container{
			Name:            s.ID,
			Image:           placeHolderImage,
			Command:         s.Entrypoint,
			Args:            s.Command,
			ImagePullPolicy: toPullPolicy(s.Pull),
			WorkingDir:      s.WorkingDir,
			SecurityContext: &v1.SecurityContext{
				Privileged: boolptr(s.Privileged),
			},
			VolumeMounts: toVolumeMounts(spec, s),
			Env:          toEnv(s),
			// TODO(bradrydzewski) revisit how we want to pass sensitive data
			// to the pipeline contianers.
			// EnvFrom:      toEnvFrom(s),
		}

		containers = append(containers, container)
	}

	return containers
}

func toEnv(step *Step) []v1.EnvVar {
	var envVars []v1.EnvVar

	for k, v := range step.Envs {
		envVars = append(envVars, v1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	// TODO(bradrydzewski) revisit how we want to pass sensitive data
	// to the pipeline contianers.
	for _, secret := range step.Secrets {
		envVars = append(envVars, v1.EnvVar{
			Name:  secret.Env,
			Value: string(secret.Data),
		})
	}

	envVars = append(envVars, v1.EnvVar{
		Name: "KUBERNETES_NODE",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "spec.nodeName",
			},
		},
	})

	return envVars
}

func toEnvFrom(step *Step) []v1.EnvFromSource {
	return []v1.EnvFromSource{
		{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: step.ID,
				},
			},
		},
	}
}

func toSecret(step *Step) *v1.Secret {
	stringData := make(map[string]string)
	for _, secret := range step.Secrets {
		stringData[secret.Env] = string(secret.Data)
	}

	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: step.ID,
		},
		Type:       "Opaque",
		StringData: stringData,
	}
}

func toVolumeMounts(spec *Spec, step *Step) []v1.VolumeMount {
	var volumeMounts []v1.VolumeMount
	for _, v := range step.Volumes {
		id, ok := lookupVolumeID(spec, v.Name)
		if !ok {
			continue
		}
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      id,
			MountPath: v.Path,
		})
	}
	return volumeMounts
}

// LookupVolume is a helper function that will lookup
// the id for a volume.
func lookupVolumeID(spec *Spec, name string) (string, bool) {
	for _, v := range spec.Volumes {
		if v.EmptyDir != nil && v.EmptyDir.Name == name {
			return v.EmptyDir.ID, true
		}

		if v.HostPath != nil && v.HostPath.Name == name {
			return v.HostPath.ID, true
		}
	}

	return "", false
}

func toPullPolicy(policy PullPolicy) v1.PullPolicy {
	switch policy {
	case PullAlways:
		return v1.PullAlways
	case PullNever:
		return v1.PullNever
	case PullIfNotExists:
		return v1.PullIfNotPresent
	default:
		return v1.PullIfNotPresent
	}
}

func int64ptr(v int64) *int64 {
	return &v
}

func boolptr(v bool) *bool {
	return &v
}

func stringptr(v string) *string {
	return &v
}