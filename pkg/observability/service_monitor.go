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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceMonitor creates a ServiceMonitor CR
func ServiceMonitor(
	name string,
	namespace string,
	labels map[string]string,
	selector map[string]string,
	scrape_interval string,
) *monv1.ServiceMonitor {
	serviceMonitor := &monv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: monv1.ServiceMonitorSpec{
			Endpoints: []monv1.Endpoint{
				{
					Interval: monv1.Duration(scrape_interval),
					MetricRelabelConfigs: []*monv1.RelabelConfig{
						{
							Action:       "labeldrop",
							Regex:        "pod",
							SourceLabels: []monv1.LabelName{},
						},
						{
							Action:       "labeldrop",
							Regex:        "namespace",
							SourceLabels: []monv1.LabelName{},
						},
						{
							Action:       "labeldrop",
							Regex:        "instance",
							SourceLabels: []monv1.LabelName{},
						},
						{
							Action:       "labeldrop",
							Regex:        "job",
							SourceLabels: []monv1.LabelName{},
						},
						{
							Action:       "labeldrop",
							Regex:        "publisher",
							SourceLabels: []monv1.LabelName{},
						},
					},
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: selector,
			},
		},
	}
	return serviceMonitor
}
