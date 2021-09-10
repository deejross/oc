package tools

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
|---------------------------------------------------- NOTICE ---------------------------------------------|
| Once the API gets merged into openshift/api, this file should get deleted, as it's just a copy for now. |
|---------------------------------------------------------------------------------------------------------|
*/

// CLIToolSpec defines the desired state of CLITool
type CLIToolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Description of the CLI tool
	// +optional
	Description string `json:"description,omitempty"`

	// Binaries for the CLI tool
	// +required
	Binaries []CLIToolBinary `json:"binaries,omitempty"`
}

// CLIToolBinary defines per-OS and per-Arch binaries for the given tool.
type CLIToolBinary struct {
	// OS is the operating system for the given binary (i.e. linux, darwin, windows)
	// +required
	OS string `json:"os,omitempty"`

	// Architecture is the CPU architecture for given binary (i.e. amd64, arm64)
	// +required
	Architecture string `json:"arch,omitempty"`

	// Image containing CLI tool
	// +required
	Image string `json:"image,omitempty"`

	// ImagePullSecret to use when connecting to an image registry that requires authentication
	// +optional
	ImagePullSecret string `json:"imagePullSecret,omitempty"`

	// Path is the location within the image where the CLI tool can be found
	// +required
	Path string `json:"path,omitempty"`
}

// CLIToolStatus defines the observed state of CLITool
type CLIToolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CLITool is the Schema for the clitools API
type CLITool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CLIToolSpec   `json:"spec,omitempty"`
	Status CLIToolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CLIToolList contains a list of CLITool
type CLIToolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CLITool `json:"items"`
}
