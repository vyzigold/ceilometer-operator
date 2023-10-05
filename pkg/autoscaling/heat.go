/*
Copyright 2022.

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

package autoscaling

import (
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Heat deploys the heat instance deployed by autoscaling
func Heat(
	instance *telemetryv1.Autoscaling,
	labels map[string]string,
) *heatv1.Heat {
	heat := &heatv1.Heat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Heat.InstanceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: *instance.Spec.Heat.Template,
	}
	return heat
}
