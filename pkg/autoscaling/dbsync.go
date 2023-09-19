package autoscaling

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	autoscalingv1beta1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// TODO: Change this ugliness
	dbSyncCommand = "/usr/local/bin/container-scripts/init.sh && mkdir -p /var/lib/kolla/config_files && cp /var/lib/config-data/merged/aodh-dbsync-config.json /var/lib/kolla/config_files/config.json && /usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
)

// DbSyncJob func
func DbSyncJob(instance *autoscalingv1beta1.Autoscaling, labels map[string]string) *batchv1.Job {
	args := []string{"-c"}
	args = append(args, dbSyncCommand)

	runAsUser := int64(0)
	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["KOLLA_BOOTSTRAP"] = env.SetValue("TRUE")
	aodhPassword := []corev1.EnvVar{
		{
			Name: "AodhPassword",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.Aodh.Secret,
					},
					Key: instance.Spec.Aodh.PasswordSelectors.Service,
				},
			},
		},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName + "-db-sync",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: instance.RbacResourceName(),
					Containers: []corev1.Container{
						{
							Name: ServiceName + "-db-sync",
							Command: []string{
								"/bin/bash",
							},
							Args:  args,
							Image: instance.Spec.Aodh.APIImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env:          env.MergeEnvs(aodhPassword, envVars),
							VolumeMounts: getDBSyncVolumeMounts(),
						},
					},
					Volumes: getVolumes(ServiceName),
				},
			},
		},
	}

	return job
}
