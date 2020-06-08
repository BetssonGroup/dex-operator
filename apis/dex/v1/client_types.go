/*


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

// +kubebuilder:validation:Required

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClientSpec defines the desired state of Client
type ClientSpec struct {
	// +kubebuilder:validation:MinLength=4

	// The name of the oidc config
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:MinLength=2
	// +kubebuilder:validation:Required

	// The shared oidc secret
	Secret string `json:"secret,omitempty"`

	// +optional

	// Sets the public flag
	Public bool `json:"public,omitempty"`

	// Redirect URIs
	RedirectURIs []string `json:"redirectURIs,omitempty"`

	// +optional

	// Trusted Peers
	TrustedPeers []string `json:"trustedPeers,omitempty"`

	// +optional

	// LogoURL
	LogoURL string `json:"logoURL,omitempty"`
}

// ClientStatus defines the observed state of Client
type ClientStatus struct {

	// +optional

	State string `json:"state,omitempty"`

	// +optional

	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true

// Client is the Schema for the clients API
type Client struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientSpec   `json:"spec,omitempty"`
	Status ClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientList contains a list of Client
type ClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Client `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Client{}, &ClientList{})
}
