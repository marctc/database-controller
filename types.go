package main

import (
	"encoding/json"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/meta"
	"k8s.io/client-go/pkg/api/unversioned"
)

type DatabaseSpec struct {
	Type		string	`json:"type"`
	Secret		string	`json:"secretName"`
	Class		string	`json:"class"`
}

type DatabaseStatus struct {
	Phase	string 	`json:"phase"`
	Error	string	`json:"error,omitempty"`
	Server	string	`json:"server,omitempty"`
}

type Database struct {
	unversioned.TypeMeta `json:",inline"`
	Metadata        api.ObjectMeta `json:"metadata"`

	Spec 	DatabaseSpec 	`json:"spec"`
	Status	DatabaseStatus	`json:"status"`
}

type DatabaseList struct {
	unversioned.TypeMeta `json:",inline"`
	Metadata        unversioned.ListMeta `json:"metadata"`

	Items []Database `json:"items"`
}

// Required to satisfy Object interface
func (e *Database) GetObjectKind() unversioned.ObjectKind {
	return &e.TypeMeta
}

// Required to satisfy ObjectMetaAccessor interface
func (e *Database) GetObjectMeta() meta.Object {
	return &e.Metadata
}

// Required to satisfy Object interface
func (el *DatabaseList) GetObjectKind() unversioned.ObjectKind {
	return &el.TypeMeta
}

// Required to satisfy ListMetaAccessor interface
func (el *DatabaseList) GetListMeta() unversioned.List {
	return &el.Metadata
}

// The code below is used only to work around a known problem with third-party
// resources and ugorji. If/when these issues are resolved, the code below
// should no longer be required.

type DatabaseListCopy DatabaseList
type DatabaseCopy Database

func (e *Database) UnmarshalJSON(data []byte) error {
	tmp := DatabaseCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := Database(tmp)
	*e = tmp2
	return nil
}

func (el *DatabaseList) UnmarshalJSON(data []byte) error {
	tmp := DatabaseListCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := DatabaseList(tmp)
	*el = tmp2
	return nil
}
