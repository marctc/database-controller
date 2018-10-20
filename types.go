package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DatabaseSpec struct {
	Type   string `json:"type"`
	Secret string `json:"secretName"`
	Class  string `json:"class"`
}

type DatabaseStatus struct {
	Phase  string `json:"phase"`
	Error  string `json:"error,omitempty"`
	Server string `json:"server,omitempty"`
}

type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DatabaseSpec   `json:"spec"`
	Status DatabaseStatus `json:"status,omitempty"`
}

type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Database `json:"items"`
}
