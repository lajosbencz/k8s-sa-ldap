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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LdapServiceAccountSpec defines the desired state of LdapServiceAccount
type LdapServiceAccountSpec struct {
	// LDAP server URL
	URL string `json:"url"`

	// BindDN is the DN to bind to the LDAP server
	BindDN string `json:"bindDN"`

	// BindPW is the password for the BindDN
	BindPW string `json:"bindPW,omitempty"`

	// BaseDN is the base DN to use for the LDAP search
	BaseDN string `json:"baseDN"`

	// Filter defines the LDAP filter to apply
	Filter string `json:"filter"`

	// PollInterval specifies the interval at which to poll LDAP
	PollInterval string `json:"pollInterval,omitempty"`

	// Resource prefix
	ResourcePrefix string `json:"resourcePrefix,omitempty"`
}

// LdapServiceAccountStatus defines the observed state of LdapServiceAccount
type LdapServiceAccountStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LastSyncTime metav1.Time `json:"lastSyncTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// LdapServiceAccount is the Schema for the ldapserviceaccounts API
type LdapServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LdapServiceAccountSpec   `json:"spec,omitempty"`
	Status LdapServiceAccountStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LdapServiceAccountList contains a list of LdapServiceAccount
type LdapServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LdapServiceAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LdapServiceAccount{}, &LdapServiceAccountList{})
}
