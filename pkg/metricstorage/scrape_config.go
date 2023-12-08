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

package metricstorage

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TODO: Once we update controller-runtime to v0.15+ ,
//       k8s.io/* to v0.27+ and
//       obo-prometheus-operator to at least v0.65.1-rhobs1,
//       switch this to structured definition

// ScrapeConfig creates a ScrapeConfig CR
func ScrapeConfig(
	name string,
	namespace string,
	labels map[string]string,
	selector map[string]string,
	scrape_interval string,
) *unstructured.Unstructured {
	scrapeConfig := &unstructured.Unstructured{}
	scrapeConfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.rhobs",
		Version: "v1alpha1",
		Kind:    "ScrapeConfig",
	})
	scrapeConfig.SetName(name)
	scrapeConfig.SetNamespace(namespace)
	scrapeConfig.SetLabels(labels)
	// TODO: set spec
	// SetUnstructuredContent
	return scrapeConfig
}
