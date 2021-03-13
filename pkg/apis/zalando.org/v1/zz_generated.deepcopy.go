// +build !ignore_autogenerated

/*
Copyright 2021 The Kubernetes Authors.

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchDataSet) DeepCopyInto(out *ElasticsearchDataSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchDataSet.
func (in *ElasticsearchDataSet) DeepCopy() *ElasticsearchDataSet {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchDataSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ElasticsearchDataSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchDataSetList) DeepCopyInto(out *ElasticsearchDataSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ElasticsearchDataSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchDataSetList.
func (in *ElasticsearchDataSetList) DeepCopy() *ElasticsearchDataSetList {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchDataSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ElasticsearchDataSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchDataSetScaling) DeepCopyInto(out *ElasticsearchDataSetScaling) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchDataSetScaling.
func (in *ElasticsearchDataSetScaling) DeepCopy() *ElasticsearchDataSetScaling {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchDataSetScaling)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchDataSetSpec) DeepCopyInto(out *ElasticsearchDataSetSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	in.Template.DeepCopyInto(&out.Template)
	if in.Scaling != nil {
		in, out := &in.Scaling, &out.Scaling
		*out = new(ElasticsearchDataSetScaling)
		**out = **in
	}
	if in.VolumeClaimTemplates != nil {
		in, out := &in.VolumeClaimTemplates, &out.VolumeClaimTemplates
		*out = make([]corev1.PersistentVolumeClaim, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchDataSetSpec.
func (in *ElasticsearchDataSetSpec) DeepCopy() *ElasticsearchDataSetSpec {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchDataSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchDataSetStatus) DeepCopyInto(out *ElasticsearchDataSetStatus) {
	*out = *in
	if in.ObservedGeneration != nil {
		in, out := &in.ObservedGeneration, &out.ObservedGeneration
		*out = new(int64)
		**out = **in
	}
	if in.LastScaleUpStarted != nil {
		in, out := &in.LastScaleUpStarted, &out.LastScaleUpStarted
		*out = (*in).DeepCopy()
	}
	if in.LastScaleUpEnded != nil {
		in, out := &in.LastScaleUpEnded, &out.LastScaleUpEnded
		*out = (*in).DeepCopy()
	}
	if in.LastScaleDownStarted != nil {
		in, out := &in.LastScaleDownStarted, &out.LastScaleDownStarted
		*out = (*in).DeepCopy()
	}
	if in.LastScaleDownEnded != nil {
		in, out := &in.LastScaleDownEnded, &out.LastScaleDownEnded
		*out = (*in).DeepCopy()
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchDataSetStatus.
func (in *ElasticsearchDataSetStatus) DeepCopy() *ElasticsearchDataSetStatus {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchDataSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchMetric) DeepCopyInto(out *ElasticsearchMetric) {
	*out = *in
	in.Timestamp.DeepCopyInto(&out.Timestamp)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchMetric.
func (in *ElasticsearchMetric) DeepCopy() *ElasticsearchMetric {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchMetric)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchMetricSet) DeepCopyInto(out *ElasticsearchMetricSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Metrics != nil {
		in, out := &in.Metrics, &out.Metrics
		*out = make([]ElasticsearchMetric, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchMetricSet.
func (in *ElasticsearchMetricSet) DeepCopy() *ElasticsearchMetricSet {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchMetricSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ElasticsearchMetricSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchMetricSetList) DeepCopyInto(out *ElasticsearchMetricSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ElasticsearchMetricSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchMetricSetList.
func (in *ElasticsearchMetricSetList) DeepCopy() *ElasticsearchMetricSetList {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchMetricSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ElasticsearchMetricSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EmbeddedObjectMeta) DeepCopyInto(out *EmbeddedObjectMeta) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EmbeddedObjectMeta.
func (in *EmbeddedObjectMeta) DeepCopy() *EmbeddedObjectMeta {
	if in == nil {
		return nil
	}
	out := new(EmbeddedObjectMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodTemplateSpec) DeepCopyInto(out *PodTemplateSpec) {
	*out = *in
	in.EmbeddedObjectMeta.DeepCopyInto(&out.EmbeddedObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodTemplateSpec.
func (in *PodTemplateSpec) DeepCopy() *PodTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(PodTemplateSpec)
	in.DeepCopyInto(out)
	return out
}
