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

	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
)

const (
	AodhApiContainerImage = "quay.io/podified-antelope-centos9/openstack-aodh-api:current-podified"
	AodhEvaluatorContainerImage = "quay.io/podified-antelope-centos9/openstack-aodh-evaluator:current-podified"
	AodhNotifierContainerImage = "quay.io/podified-antelope-centos9/openstack-aodh-notifier:current-podified"
	AodhListenerContainerImage = "quay.io/podified-antelope-centos9/openstack-aodh-listener:current-podified"
)

// Prometheus defines which prometheus to use for Autoscaling
type Prometheus struct {
	// Enables the deployment of autoscaling prometheus
	// +kubebuilder:default=false
	DeployPrometheus bool `json:"deployPrometheus,omitempty"`

	// Host of user deployed prometheus if deployPrometheus is set to false
	Host string `json:"host,omitempty"`

	// Port of user deployed prometheus if deployPrometheus is set to false
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

type Aodh struct {
	// PasswordSelectors - Selectors to identify the service from the Secret
	// +kubebuilder:default:={service: CeilometerPassword}
	PasswordSelectors PasswordsSelector `json:"passwordSelector,omitempty"`

	// ServiceUser - optional username used for this service to register in keystone
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=ceilometer
	ServiceUser string `json:"serviceUser"`

	// Secret containing OpenStack password information for ceilometer
	// +kubebuilder:validation:Required
	Secret string `json:"secret"`

	// CustomServiceConfig - customize the service config using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/<service>/<service>.conf.d directory as custom.conf file.
	// +kubebuilder:default:="# add your customization here"
	CustomServiceConfig string `json:"customServiceConfig,omitempty"`

	// ConfigOverwrite - interface to overwrite default config files like e.g. logging.conf or policy.json.
	// But can also be used to add additional files. Those get added to the service config dir in /etc/<service> .
	// TODO: -> implement
	DefaultConfigOverwrite map[string]string `json:"defaultConfigOverwrite,omitempty"`

	// NetworkAttachmentDefinitions list of network attachment definitions the service pod gets attached to
	NetworkAttachmentDefinitions []string `json:"networkAttachmentDefinitions,omitempty"`

	ApiImage string `json:"apiImage"`
	EvaluatorImage string `json:"evaluatorImage"`
	NotifierImage string `json:"notifierImage"`
	ListenerImage string `json:"listenerImage"`
	InitImage string `json:"initImage"`
}

// AutoscalingSpec defines the desired state of Autoscaling
type AutoscalingSpec struct {
	// Specification of which prometheus to use for autoscaling
	Prometheus Prometheus `json:"prometheus,omitempty"`

	Aodh Aodh `json:"aodh,omitempty"`

	// Allows enabling and disabling the autoscaling feature
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`
}

// AutoscalingStatus defines the observed state of Autoscaling
type AutoscalingStatus struct {
	// ReadyCount of autoscaling instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Autoscaling is the Schema for the autoscalings API
type Autoscaling struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoscalingSpec   `json:"spec,omitempty"`
	Status AutoscalingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AutoscalingList contains a list of Autoscaling
type AutoscalingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Autoscaling `json:"items"`
}

// IsReady - returns true if Autescaling is reconciled successfully
func (instance Autoscaling) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

func init() {
	SchemeBuilder.Register(&Autoscaling{}, &AutoscalingList{})
}

// RbacConditionsSet - set the conditions for the rbac object
func (instance Autoscaling) RbacConditionsSet(c *condition.Condition) {
	instance.Status.Conditions.Set(c)
}

// RbacNamespace - return the namespace
func (instance Autoscaling) RbacNamespace() string {
	return instance.Namespace
}

// RbacResourceName - return the name to be used for rbac objects (serviceaccount, role, rolebinding)
func (instance Autoscaling) RbacResourceName() string {
	return "telemetry-" + instance.Name
}

// SetupDefaultsAutoscaling - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaultsAutoscaling() {
	// Acquire environmental defaults and initialize Telemetry defaults with them
	autoscalingDefaults := AutoscalingDefaults{
		AodhApiContainerImageURL:      util.GetEnvVar("RELATED_IMAGE_AODH_API_IMAGE_URL_DEFAULT", AodhApiContainerImage),
		AodhEvaluatorContainerImageURL:  util.GetEnvVar("RELATED_IMAGE_AODH_EVALUATOR_IMAGE_URL_DEFAULT", AodhEvaluatorContainerImage),
		AodhNotifierContainerImageURL:       util.GetEnvVar("RELATED_IMAGE_AODH_NOTIFIER_IMAGE_URL_DEFAULT", AodhNotifierContainerImage),
		AodhListenerContainerImageURL: util.GetEnvVar("RELATED_IMAGE_AODH_LISTENER_IMAGE_URL_DEFAULT", AodhListenerContainerImage),
		AodhInitContainerImageURL: util.GetEnvVar("RELATED_IMAGE_AODH_API_IMAGE_URL_DEFAULT", AodhApiContainerImage),
	}

	SetupAutoscalingDefaults(autoscalingDefaults)
}
