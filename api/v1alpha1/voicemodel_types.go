/*
Copyright 2026.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// VoiceModelType is the kind of voice task a model serves.
//
// Only "tts" is implemented; "stt" (speech-to-text) and "s2s" (speech-to-speech)
// are reserved for later versions and are rejected by the schema until the
// corresponding engine params and runtimes exist.
// +kubebuilder:validation:Enum=tts
type VoiceModelType string

const (
	// VoiceModelTypeTTS synthesizes audio from text.
	VoiceModelTypeTTS VoiceModelType = "tts"
)

// AudioFormat is the container/codec a runtime emits.
// +kubebuilder:validation:Enum=wav;opus;mp3
type AudioFormat string

const (
	// AudioFormatWAV is uncompressed PCM in a WAV container.
	AudioFormatWAV AudioFormat = "wav"
	// AudioFormatOpus is Opus in an Ogg container.
	AudioFormatOpus AudioFormat = "opus"
	// AudioFormatMP3 is MPEG-1 Audio Layer III.
	AudioFormatMP3 AudioFormat = "mp3"
)

// Condition types reported in VoiceModelStatus.
const (
	// ConditionModelDownloaded is True once the model artifacts named by
	// spec.source have been fetched into the model cache.
	ConditionModelDownloaded = "ModelDownloaded"
	// ConditionLoaded is True once the runtime has the model resident in
	// memory and can synthesize.
	ConditionLoaded = "Loaded"
	// ConditionReady is True once at least one replica is serving and
	// status.endpoint is reachable.
	ConditionReady = "Ready"
)

// ModelSource points at the model artifacts to serve.
type ModelSource struct {
	// uri locates the model artifacts. Supported schemes:
	//   hf://<org>/<repo>[@<revision>]  Hugging Face Hub
	//   s3://<bucket>/<key>             S3-compatible object storage
	//   oci://<registry>/<repo>[:<tag>] OCI artifact
	// +kubebuilder:validation:Pattern=`^(hf|s3|oci)://.+`
	// +kubebuilder:validation:MaxLength=2048
	// +required
	URI string `json:"uri"`

	// secretRef names a Secret in the VoiceModel's namespace holding
	// credentials for uri — a Hugging Face token, S3 access keys, or an OCI
	// pull secret, depending on the scheme. Omit for public artifacts.
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

// RuntimeRef references the ServingRuntime that serves this model.
type RuntimeRef struct {
	// name is the name of a ServingRuntime in the same namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`
}

// GPUSpec requests accelerators for each replica.
type GPUSpec struct {
	// count is the number of GPUs per replica.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	Count int32 `json:"count,omitempty"`

	// type is the extended resource name the device plugin advertises,
	// for example "nvidia.com/gpu" or "amd.com/gpu".
	// +kubebuilder:default="nvidia.com/gpu"
	// +optional
	Type corev1.ResourceName `json:"type,omitempty"`
}

// VoiceModelResources describes the compute each replica requests.
type VoiceModelResources struct {
	// gpu requests accelerators. Omit to serve on CPU — which is viable for
	// small TTS models such as Piper.
	// +optional
	GPU *GPUSpec `json:"gpu,omitempty"`

	// requests is the CPU/memory/ephemeral-storage requested per replica.
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`

	// limits is the CPU/memory/ephemeral-storage cap per replica. GPUs are
	// requested via gpu rather than here.
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
}

// TTSSpec holds the synthesis params passed to a text-to-speech runtime.
type TTSSpec struct {
	// voice selects the voice (also called speaker) within the model. Models
	// that ship a single voice ignore it.
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Voice string `json:"voice,omitempty"`

	// language is the BCP-47 tag the model synthesizes, for example "en-US".
	// Multilingual models use it to pick the phonemizer.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`
	// +optional
	Language string `json:"language,omitempty"`

	// sampleRate is the output sample rate in Hz. Defaults to the model's
	// native rate when unset; a different value makes the runtime resample.
	// +kubebuilder:validation:Minimum=8000
	// +kubebuilder:validation:Maximum=192000
	// +optional
	SampleRate *int32 `json:"sampleRate,omitempty"`

	// format is the audio container the endpoint returns.
	// +kubebuilder:default=wav
	// +optional
	Format AudioFormat `json:"format,omitempty"`

	// streaming serves audio in chunks as it is synthesized instead of
	// buffering the whole utterance, which cuts time-to-first-audio. The
	// referenced ServingRuntime must support it.
	// +kubebuilder:default=false
	// +optional
	Streaming bool `json:"streaming,omitempty"`
}

// VoiceModelSpec defines the desired state of VoiceModel
// +kubebuilder:validation:XValidation:rule="self.type != 'tts' || has(self.tts)",message="spec.tts is required when spec.type is tts"
type VoiceModelSpec struct {
	// type is the voice task this model serves. Only "tts" is implemented.
	// Defaulted by the API server, so readers can rely on it being set.
	// +kubebuilder:default=tts
	// +optional
	Type VoiceModelType `json:"type,omitempty"`

	// source locates the model artifacts to download and serve.
	// +required
	Source ModelSource `json:"source"`

	// runtimeRef selects the ServingRuntime that runs this model. The runtime
	// supplies the engine image and its serving contract, so the same model
	// can move between engines without changing this spec.
	// +required
	RuntimeRef RuntimeRef `json:"runtimeRef"`

	// tts holds the synthesis params. Required when type is "tts".
	// +optional
	TTS *TTSSpec `json:"tts,omitempty"`

	// replicas is the number of serving replicas. 0 scales the model down
	// without deleting it.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// resources is the compute each replica requests.
	// +optional
	Resources *VoiceModelResources `json:"resources,omitempty"`
}

// VoiceModelStatus defines the observed state of VoiceModel.
type VoiceModelStatus struct {
	// conditions represent the current state of the VoiceModel resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Condition types reported by voice-d:
	// - "ModelDownloaded": the artifacts named by spec.source are in the model cache
	// - "Loaded": the runtime holds the model in memory and can synthesize
	// - "Ready": at least one replica is serving on status.endpoint
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// endpoint is the in-cluster URL clients POST synthesis requests to. Set
	// once the Service backing this model exists.
	// +kubebuilder:validation:MaxLength=2048
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// observedGeneration is the metadata.generation this status was computed
	// from.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Runtime",type=string,JSONPath=".spec.runtimeRef.name"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=".status.endpoint"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// VoiceModel is the Schema for the voicemodels API
type VoiceModel struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of VoiceModel
	// +required
	Spec VoiceModelSpec `json:"spec"`

	// status defines the observed state of VoiceModel
	// +optional
	Status VoiceModelStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// VoiceModelList contains a list of VoiceModel
type VoiceModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []VoiceModel `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &VoiceModel{}, &VoiceModelList{})
		return nil
	})
}
