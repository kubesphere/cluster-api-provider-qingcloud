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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// QCClusterTemplateSpec defines the desired state of DOClusterTemplate.
type QCClusterTemplateSpec struct {
	Template QCClusterTemplateResource `json:"template"`
}

// QCClusterTemplateResource contains spec for DOClusterSpec.
type QCClusterTemplateResource struct {
	Spec QCClusterSpec `json:"spec"`
}

// QCClusterTemplateStatus defines the observed state of QCClusterTemplate
type QCClusterTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=qcclustertemplates,scope=Namespaced,categories=cluster-api,shortName=qcct
// +kubebuilder:storageversion

// QCClusterTemplate is the Schema for the qcclustertemplates API
type QCClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QCClusterTemplateSpec   `json:"spec,omitempty"`
	Status QCClusterTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QCClusterTemplateList contains a list of QCClusterTemplate
type QCClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QCClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QCClusterTemplate{}, &QCClusterTemplateList{})
}
