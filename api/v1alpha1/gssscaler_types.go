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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GssScalerSpec defines the desired state of GssScaler
type GssScalerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of GssScaler. Edit gssscaler_types.go to remove/update
	Name                       string `json:"name"`
	Namespace                  string `json:"namespace"`
	PollingInterval            *int32 `json:"pollingInterval"`
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds"`
}

// GssScalerStatus defines the observed state of GssScaler
type GssScalerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GssScaler is the Schema for the gssscalers API
type GssScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GssScalerSpec   `json:"spec,omitempty"`
	Status GssScalerStatus `json:"status,omitempty"`
}

// IsBeingDeleted returns true if a deletion timestamp is set
func (r *GssScaler) IsBeingDeleted() bool {
	return !r.ObjectMeta.DeletionTimestamp.IsZero()
}

const FinalizerName = "GssScaler.dev.xingzheai.cn"

// HasFinalizer returns true if the item has the specified finalizer
func (r *GssScaler) HasFinalizer(finalizerName string) bool {
	return containsString(r.ObjectMeta.Finalizers, finalizerName)
}

// AddFinalizer adds the specified finalizer
func (r *GssScaler) AddFinalizer(finalizerName string) {
	r.ObjectMeta.Finalizers = append(r.ObjectMeta.Finalizers, finalizerName)
}

// RemoveFinalizer removes the specified finalizer
func (r *GssScaler) RemoveFinalizer(finalizerName string) {
	r.ObjectMeta.Finalizers = removeString(r.ObjectMeta.Finalizers, finalizerName)
}

//+kubebuilder:object:root=true

// GssScalerList contains a list of GssScaler
type GssScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GssScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GssScaler{}, &GssScalerList{})
}
