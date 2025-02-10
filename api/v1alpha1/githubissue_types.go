/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GithubIssueSpec defines the desired state of GithubIssue.
type GithubIssueSpec struct {
	// Repo is the URL of the GitHub repository of the GithubIssue.
	Repo string `json:"repo"`
	// Title is the title of the GithubIssue.
	Title string `json:"title"`
	// Description is the description of the GithubIssue.
	Description string `json:"description"`
}

// GithubIssueStatus defines the observed state of GithubIssue.
type GithubIssueStatus struct {
	// Conditions represents the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GithubIssue is the Schema for the githubissues API.
// +kubebuilder:resource:scope=Namespaced,shortName=ghi
// +kubebuilder:printcolumn:name="Repo",type=string,JSONPath=".spec.repo",description="GitHub Repository",priority=0
// +kubebuilder:printcolumn:name="Title",type=string,JSONPath=".spec.title",description="Issue Title",priority=0
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=".spec.description",description="Issue Description",priority=0
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.conditions[?(@.type=='Open')].status",description="Open status",priority=0
// +kubebuilder:object:generate=true
type GithubIssue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GithubIssueSpec   `json:"spec,omitempty"`
	Status GithubIssueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GithubIssueList contains a list of GithubIssue.
// +kubebuilder:object:generate=true
type GithubIssueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GithubIssue `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GithubIssue{}, &GithubIssueList{})
}
