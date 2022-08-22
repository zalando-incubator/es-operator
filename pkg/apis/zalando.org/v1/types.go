package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// ElasticsearchDataSet describes an Elasticsearch dataset which is operated
// by the es-operator.
// +k8s:deepcopy-gen=true
// +kubebuilder:resource:categories="all",shortName=eds
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.spec.replicas`,description="The desired number of replicas for the stateful set"
// +kubebuilder:printcolumn:name="Current",type=integer,JSONPath=`.status.replicas`,description="The current number of replicas for the stateful set"
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.hpaReplicas,statuspath=.status.hpaReplicas,selectorpath=.status.selector
type ElasticsearchDataSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ElasticsearchDataSetSpec `json:"spec"`
	// +optional
	Status ElasticsearchDataSetStatus `json:"status"`
}

// ElasticsearchDataSetSpec is the spec part of the Elasticsearch dataset.
// +k8s:deepcopy-gen=true
type ElasticsearchDataSetSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`

	// Number of pods specified by the HPA.
	// +optional
	HpaReplicas *int32 `json:"hpaReplicas,omitempty" protobuf:"varint,1,opt,name=hpaReplicas"`

	// Exclude management of System Indices on this Data Set. Defaults to false
	// +optional
	ExcludeSystemIndices bool `json:"excludeSystemIndices"`

	// SkipDraining determines whether pods of the EDS should be drained
	// before termination or not. Defaults to false
	// +optional
	SkipDraining bool `json:"skipDraining"`

	// // serviceName is the name of the service that governs this StatefulSet.
	// // This service must exist before the StatefulSet, and is responsible for
	// // the network identity of the set. Pods get DNS/hostnames that follow the
	// // pattern: pod-specific-string.serviceName.default.svc.cluster.local
	// // where "pod-specific-string" is managed by the StatefulSet controller.
	// ServiceName string `json:"serviceName" protobuf:"bytes,5,opt,name=serviceName"`

	// Template describes the pods that will be created.
	Template PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`

	// Scaling describes the scaling properties
	Scaling *ElasticsearchDataSetScaling `json:"scaling,omitempty"`

	// Template describe the volumeClaimTemplates
	VolumeClaimTemplates []PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty" protobuf:"bytes,4,rep,name=volumeClaimTemplates"`
}

// PersistentVolumeClaim is a user's request for and claim to a persistent volume
// +k8s:deepcopy-gen=true
type PersistentVolumeClaim struct {
	EmbeddedObjectMetaWithName `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired characteristics of a volume requested by a pod author.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	// +optional
	Spec v1.PersistentVolumeClaimSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// EmbeddedObjectWithName defines the metadata which can be attached
// to a resource. It's a slimmed down version of metav1.ObjectMeta only
// containing name, labels and annotations.
// +k8s:deepcopy-gen=true
type EmbeddedObjectMetaWithName struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}

// EmbeddedObject defines the metadata which can be attached
// to a resource. It's a slimmed down version of metav1.ObjectMeta only
// containing labels and annotations.
// +k8s:deepcopy-gen=true
type EmbeddedObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}

// PodTemplateSpec describes the data a pod should have when created from a template
// +k8s:deepcopy-gen=true
type PodTemplateSpec struct {
	// Object's metadata.
	// +optional
	EmbeddedObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec v1.PodSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// ElasticsearchDataSetScaling is the scaling section of the ElasticsearchDataSet
// resource.
// +k8s:deepcopy-gen=true
type ElasticsearchDataSetScaling struct {
	// +optional
	Enabled bool `json:"enabled"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReplicas int32 `json:"minReplicas"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxReplicas int32 `json:"maxReplicas"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinIndexReplicas int32 `json:"minIndexReplicas"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxIndexReplicas int32 `json:"maxIndexReplicas"`
	// +kubebuilder:validation:Minimum=1
	// +optional
	MinShardsPerNode int32 `json:"minShardsPerNode"`
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxShardsPerNode int32 `json:"maxShardsPerNode"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	ScaleUpCPUBoundary int32 `json:"scaleUpCPUBoundary"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	ScaleUpThresholdDurationSeconds int64 `json:"scaleUpThresholdDurationSeconds"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	ScaleUpCooldownSeconds int64 `json:"scaleUpCooldownSeconds"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	ScaleDownCPUBoundary int32 `json:"scaleDownCPUBoundary"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	ScaleDownThresholdDurationSeconds int64 `json:"scaleDownThresholdDurationSeconds"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	ScaleDownCooldownSeconds int64 `json:"scaleDownCooldownSeconds"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	DiskUsagePercentScaledownWatermark int32 `json:"diskUsagePercentScaledownWatermark"`
}

// ElasticsearchDataSetStatus is the status section of the ElasticsearchDataSet
// resource.
// +k8s:deepcopy-gen=true
type ElasticsearchDataSetStatus struct {
	// observedGeneration is the most recent generation observed for this
	// ElasticsearchDataSet. It corresponds to the ElasticsearchDataSets
	// generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`
	// Replicas is the number of Pods by the underlying StatefulSet.
	Replicas int32 `json:"replicas" protobuf:"varint,2,opt,name=replicas"`

	// HpaReplicas is the number of Pods determined by the HPA metrics.
	HpaReplicas int32 `json:"hpaReplicas" protobuf:"varint,2,opt,name=hpaReplicas"`

	// Hpa selector
	Selector string `json:"selector"`

	LastScaleUpStarted   *metav1.Time `json:"lastScaleUpStarted,omitempty"`
	LastScaleUpEnded     *metav1.Time `json:"lastScaleUpEnded,omitempty"`
	LastScaleDownStarted *metav1.Time `json:"lastScaleDownStarted,omitempty"`
	LastScaleDownEnded   *metav1.Time `json:"lastScaleDownEnded,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElasticsearchDataSetList is a list of ElasticsearchDataSets.
// +k8s:deepcopy-gen=true
type ElasticsearchDataSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ElasticsearchDataSet `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElasticsearchMetricSet is the metrics holding section of the ElasticsearchDataSet
// resource.
// +k8s:deepcopy-gen=true
type ElasticsearchMetricSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Metrics           []ElasticsearchMetric `json:"metrics"`
}

// ElasticsearchMetric is the single metric sample of the ElasticsearchDataSet
// resource.
// +k8s:deepcopy-gen=true
type ElasticsearchMetric struct {
	Timestamp metav1.Time `json:"timestamp"`
	Value     int32       `json:"value"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElasticsearchDataSetList is a list of ElasticsearchDataSets.
// +k8s:deepcopy-gen=true
type ElasticsearchMetricSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ElasticsearchMetricSet `json:"items"`
}
