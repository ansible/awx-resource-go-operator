/*
Copyright 2023.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AnsibleJobSpec defines the desired state of AnsibleJob
type AnsibleJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ExtraVars            map[string]string `json:"extraVars,omitempty"`
	Inventory            string            `json:"inventory,omitempty"`
	JobTemplateName      string            `json:"jobTemplateName,omitempty"`
	WorkflowTemplateName string            `json:"workflowTemplateName,omitempty"`
	RunnerImage          string            `json:"runnerImage,omitempty"`
	RunnerVersion        string            `json:"runnerVersion,omitempty"`
	//+kubebuilder:validation:Required
	TowerAuthSecret string `json:"towerAuthSecret"`
	JobTTL          *int   `json:"jobTTL,omitempty"`
	JobTags         string `json:"jobTags,omitempty"`
	SkipTags        string `json:"skipTags,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AnsibleJob is the Schema for the ansiblejobs API
type AnsibleJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnsibleJobSpec `json:"spec,omitempty"`
	Status v1.PodStatus   `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AnsibleJobList contains a list of AnsibleJob
type AnsibleJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AnsibleJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AnsibleJob{}, &AnsibleJobList{})
}
