/*
Copyright 2024.

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

package v1alpha1

import (
	kcore "k8s.io/api/core/v1"
	kmeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodSpec struct {
	// +kubebuilder:validation:Required
	// Validate docker inage name (with optional tag and registry address)
	// +kubebuilder:validation:Pattern=`(?:[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*\/)?(?:[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*\/)*[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*(:[a-zA-Z0-9_.-]+)?`
	Image string `json:"image"`

	// +kubebuilder:validation:Optional
	Resources *kcore.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	ReadinessProbe *kcore.Probe `json:"readiness_probe,omitempty"`

	// +kubebuilder:validation:Optional
	LivenessProbe *kcore.Probe `json:"liveness_probe,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=Default
	DNSPolicy kcore.DNSPolicy `json:"dns_policy"`
}

type LocalstackInstanceSpec struct {
	PodSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Debug bool `json:"debug"`

	// +kubebuilder:validation:Optional
	LambdaEnvironmentTimeout *kmeta.Duration `json:"lambda_environment_timeout"`
}

type GDCEnvSpec struct {
	PodSpec `json:",inline"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LocalstackSpec defines the desired state of Localstack
type LocalstackSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=none;coredns
	DNSProvider string `json:"dns_provider"`

	// +kubebuilder:validation:Required
	DnsConfigName string `json:"dns_config_name"`

	// +kubebuilder:validation:Required
	DnsConfigNamespace string `json:"dns_config_namespace"`

	// +kubebuilder:validation:Required
	LocalstackInstanceSpec LocalstackInstanceSpec `json:"localstack,omitempty"`

	// +kubebuilder:validation:Optional
	GDCEnvSpec *GDCEnvSpec `json:"gdc,omitempty"`
}

// LocalstackStatus defines the observed state of Localstack
type LocalstackStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ReadyLocalstack bool    `json:"ready_localstack"`
	ReadyDev        bool    `json:"ready_dev"`
	IP              *string `json:"ip,omitempty"`
	DNS             *string `json:"dns,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".status.ready_localstack",name="Ready LS",type="string"
//+kubebuilder:printcolumn:JSONPath=".status.ready_dev",name="Ready Dev",type="string"
//+kubebuilder:printcolumn:JSONPath=".status.ip",name="Cluster IP",type="string"
//+kubebuilder:printcolumn:JSONPath=".status.dns",name="Cluster DNS",type="string"

// Localstack is the Schema for the localstacks API
type Localstack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocalstackSpec   `json:"spec,omitempty"`
	Status LocalstackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LocalstackList contains a list of Localstack
type LocalstackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Localstack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Localstack{}, &LocalstackList{})
}
