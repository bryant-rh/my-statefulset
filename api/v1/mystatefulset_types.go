/*
Copyright 2024 bryant-rh.

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

package v1

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MyStatefulsetSpec defines the desired state of MyStatefulset
type MyStatefulsetSpec struct {
	// Replicas is the desired number of replicas of the given Template.
	// These are replicas in the sense that they are instantiations of the
	// same Template, but individual replicas also have a consistent identity.
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`

	// ServiceName is the name of the service that governs this StatefulSet.
	// This service must exist before the StatefulSet, and is responsible for
	// the network identity of the set.
	// +kubebuilder:validation:Required
	ServiceName string `json:"serviceName"`

	// Selector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// +kubebuilder:validation:Required
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected. Each pod stamped out by the StatefulSet
	// will fulfill this Template, but have a unique identity from the rest
	// of the StatefulSet.
	// +optional
	// +kubebuilder:validation:Required
	Template PodTemplateSpec `json:"template,omitempty"`
	// VolumeClaimTemplates is a list of claims that pods are allowed to reference.
	// +optional
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// UpdateStrategy indicates the StatefulSetUpdateStrategy that will be
	// employed to update Pods in the StatefulSet when a revision is made to
	// Template.
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`

	// MinReadySeconds is the minimum number of seconds for which a newly created pod should be ready
	// without any of its container crashing, for it to be considered available.
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`
}

type PodTemplateSpec struct {
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XPreserveUnknownFields
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec v1.PodSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// UpdateStrategy defines the strategy used for updating pods in a StatefulSet.
type UpdateStrategy struct {
	Type          StatefulSetUpdateStrategyType     `json:"type,omitempty"` // +kubebuilder:validation:Enum=RollingUpdate;OnDelete
	RollingUpdate *RollingUpdateStatefulSetStrategy `json:"rollingUpdate,omitempty"`
}

// RollingUpdateStatefulSetStrategy is used to control the rolling update of a StatefulSet.
type RollingUpdateStatefulSetStrategy struct {
	Partition *int32 `json:"partition,omitempty"` // Default is 0.
}

// MyStatefulsetStatus defines the observed state of MyStatefulset.
type MyStatefulsetStatus struct {
	CurrentGeneration int64 `json:"currentGeneration,omitempty"`
	Replicas          int32 `json:"replicas"`
	ReadyReplicas     int32 `json:"readyReplicas"`
	CurrentReplicas   int32 `json:"currentReplicas"`
	UpdatedReplicas   int32 `json:"updatedReplicas"`
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ObservedGeneration is the most recent generation observed for this StatefulSet
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
//+kubebuilder:resource:path=mystatefulsets,scope=Namespaced,shortName=kms
//+kubebuilder:printcolumn:name="DESIRED",type="integer",JSONPath=".spec.replicas",description="Desired number of pods"
//+kubebuilder:printcolumn:name="READY",type="integer",JSONPath=".status.readyReplicas",description="Number of pods ready"
//+kubebuilder:printcolumn:name="CURRENT",type="integer",JSONPath=".status.currentReplicas",description="Current number of pods"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="UPDATED",type="integer",JSONPath=".status.updatedReplicas",description="Number of pods updated"
//+kubebuilder:printcolumn:name="AVAILABLE",type="integer",JSONPath=".status.availableReplicas",description="Number of pods available"
//+groupName=apps.mystatefulset.com

// MyStatefulset is the Schema for the mystatefulsets API
type MyStatefulset struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MyStatefulsetSpec   `json:"spec,omitempty"`
	Status            MyStatefulsetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MyStatefulsetList contains a list of MyStatefulset
type MyStatefulsetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MyStatefulset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MyStatefulset{}, &MyStatefulsetList{})
}

// Validate方法用于对MyStatefulset进行基本的验证。
func (m *MyStatefulset) Validate() error {
	if m.Spec.Replicas < 0 {
		return fmt.Errorf("replicas must be zero or greater")
	}
	if m.Spec.ServiceName == "" {
		return fmt.Errorf("serviceName is required")
	}
	if m.Spec.Selector == nil {
		return fmt.Errorf("selector is required")
	}
	return nil
}

// func (m *MyStatefulset) SetDefault() {
// 	if m.Spec.UpdateStrategy.Type == "" {
// 		m.Spec.UpdateStrategy.Type = RollingUpdateStatefulSetStrategyType
// 	}
// 	if m.Spec.UpdateStrategy.Type == RollingUpdateStatefulSetStrategyType &&
// 		m.Spec.UpdateStrategy.RollingUpdate == nil {
// 		m.Spec.UpdateStrategy.RollingUpdate = &RollingUpdateStatefulSetStrategy{}
// 	}
// 	if m.Spec.UpdateStrategy.RollingUpdate != nil &&
// 		m.Spec.UpdateStrategy.RollingUpdate.Partition == nil {
// 		partition := int32(0)
// 		m.Spec.UpdateStrategy.RollingUpdate.Partition = &partition
// 	}
// }

// StatefulSetUpdateStrategyType is a string enumeration type that enumerates
// all possible update strategies for the StatefulSet controller.
type StatefulSetUpdateStrategyType string

const (
	// RollingUpdateStatefulSetStrategyType indicates that update will be
	// applied to all Pods in the StatefulSet with respect to the StatefulSet
	// ordering constraints.
	RollingUpdateStatefulSetStrategyType StatefulSetUpdateStrategyType = "RollingUpdate"
	// OnDeleteStatefulSetStrategyType triggers the legacy behavior. Version
	// tracking and ordered rolling restarts are disabled.
	OnDeleteStatefulSetStrategyType StatefulSetUpdateStrategyType = "OnDelete"
)
