package v1alpha1

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending = "PENDING"
	PhaseRunning = "RUNNING"
	PhaseDone    = "DONE"
)

type AtSpec struct {
	Schedule string `json:"schedule,omitempty"`
	Command  string `json:"command,omitempty"`
}

type AtStatus struct {
	Phase string `json:"phase,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interface=k8s.io/apimachinery/pkg/runtime.Object

type At struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AtSpec   `json:"spec"`
	Status AtStatus `json:"status"`
}

// +k8s:deepcopy-gen:interface=k8s.io/apimachinery/pkg/runtime.Object

type AtList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []At `json:"items"`
}
