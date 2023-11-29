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

package prometheus

import (
	monv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	obov1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MonitoringStack creates a MonitoringStack CR
func MonitoringStack(
	name string,
	namespace string,
	labels map[string]string,
	alertmanagerDisabled bool,
	prometheusReplicas int32,
	prometheusRetention string,
) *obov1.MonitoringStack {
	monitoringStack := &obov1.MonitoringStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: obov1.MonitoringStackSpec{
			AlertmanagerConfig: obov1.AlertmanagerConfig{
				Disabled: alertmanagerDisabled,
			},
			PrometheusConfig: &obov1.PrometheusConfig{
				Replicas: &prometheusReplicas,
			},
			Retention:         monv1.Duration(prometheusRetention),
			NamespaceSelector: &metav1.LabelSelector{},
			ResourceSelector:  &metav1.LabelSelector{},
		},
	}
	return monitoringStack
}
