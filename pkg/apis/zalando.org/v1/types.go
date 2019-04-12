package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElasticsearchDataSet describes an Elasticsearch dataset which is operated
// by the es-operator.
// +k8s:deepcopy-gen=true
type ElasticsearchDataSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElasticsearchDataSetSpec   `json:"spec"`
	Status ElasticsearchDataSetStatus `json:"status"`
}

// ElasticsearchDataSetSpec is the spec part of the Elasticsearch dataset.
// +k8s:deepcopy-gen=true
type ElasticsearchDataSetSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`

	// // serviceName is the name of the service that governs this StatefulSet.
	// // This service must exist before the StatefulSet, and is responsible for
	// // the network identity of the set. Pods get DNS/hostnames that follow the
	// // pattern: pod-specific-string.serviceName.default.svc.cluster.local
	// // where "pod-specific-string" is managed by the StatefulSet controller.
	// ServiceName string `json:"serviceName" protobuf:"bytes,5,opt,name=serviceName"`

	// Template describes the pods that will be created.
	Template v1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`

	// Scaling describes the scaling properties
	Scaling *ElasticsearchDataSetScaling `json:"scaling,omitempty"`

	// Template describe the volumeClaimTemplates
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty" protobuf:"bytes,4,rep,name=volumeClaimTemplates"`
}

// ElasticsearchDataSetScaling is the scaling section of the ElasticsearchDataSet
// resource.
// +k8s:deepcopy-gen=true
type ElasticsearchDataSetScaling struct {
	Enabled                            bool    `json:"enabled"`
	MinReplicas                        int32   `json:"minReplicas"`
	MaxReplicas                        int32   `json:"maxReplicas"`
	MinIndexReplicas                   int32   `json:"minIndexReplicas"`
	MaxIndexReplicas                   int32   `json:"maxIndexReplicas"`
	MinShardsPerNode                   int32   `json:"minShardsPerNode"`
	MaxShardsPerNode                   int32   `json:"maxShardsPerNode"`
	ScaleUpCPUBoundary                 int32   `json:"scaleUpCPUBoundary"`
	ScaleUpThresholdDurationSeconds    int64   `json:"scaleUpThresholdDurationSeconds"`
	ScaleUpCooldownSeconds             int64   `json:"scaleUpCooldownSeconds"`
	ScaleDownCPUBoundary               int32   `json:"scaleDownCPUBoundary"`
	ScaleDownThresholdDurationSeconds  int64   `json:"scaleDownThresholdDurationSeconds"`
	ScaleDownCooldownSeconds           int64   `json:"scaleDownCooldownSeconds"`
	DiskUsagePercentScaledownWatermark float64 `json:"diskUsagePercentScaledownWatermark"`
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
