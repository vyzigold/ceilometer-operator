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

package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	obov1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"

)

type PersistentStorage struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="20G"
	PvcStorageRequest string `json:"pvcStorageRequest,omitempty"`

	// +kubebuilder:validation:Optional
	PvcStorageSelector metav1.LabelSelector `json:"pvcStorageSelector,omitempty"`

	// +kubebuilder:validation:Optional
	PvcStorageClass string `json:"pvcStorageClass,omitempty"`
}

type Storage struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=persistent;ephemeral
	// +kubebuilder:default=persistent
	Strategy string `json:"strategy,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="24h"
	Retention string `json:"retention,omitempty"`

	// +kubebuilder:validation:Optional
	Persistent PersistentStorage `json:"persistent,omitempty"`
}

// MetricStorageSpec defines the desired state of MetricStorage
type MetricStorageSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=prometheus
	// +kubebuilder:default=prometheus
	Type string `json:"type,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	AlertingEnabled bool `json:"alertingEnabled,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="30s"
	ScrapeInterval string `json:"scrapeInterval,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={strategy: persistent, retention: "24h", persistent: {pvcStorageRequest: "20G"}}
	Storage `json:"storage,omitempty"`

	// +kubebuilder:validation:Optional
	CustomMonitoringStack obov1.MonitoringStackSpec `json:"customMonitoringStack,omitempty"`
}

// MetricStorageStatus defines the observed state of MetricStorage
type MetricStorageStatus struct {
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MetricStorage is the Schema for the metricstorages API
type MetricStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetricStorageSpec   `json:"spec,omitempty"`
	Status MetricStorageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MetricStorageList contains a list of MetricStorage
type MetricStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetricStorage{}, &MetricStorageList{})
}
