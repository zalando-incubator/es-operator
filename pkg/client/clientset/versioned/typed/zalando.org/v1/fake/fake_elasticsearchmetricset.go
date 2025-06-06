/*
Copyright 2025 The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	zalandoorgv1 "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned/typed/zalando.org/v1"
	gentype "k8s.io/client-go/gentype"
)

// fakeElasticsearchMetricSets implements ElasticsearchMetricSetInterface
type fakeElasticsearchMetricSets struct {
	*gentype.FakeClientWithList[*v1.ElasticsearchMetricSet, *v1.ElasticsearchMetricSetList]
	Fake *FakeZalandoV1
}

func newFakeElasticsearchMetricSets(fake *FakeZalandoV1, namespace string) zalandoorgv1.ElasticsearchMetricSetInterface {
	return &fakeElasticsearchMetricSets{
		gentype.NewFakeClientWithList[*v1.ElasticsearchMetricSet, *v1.ElasticsearchMetricSetList](
			fake.Fake,
			namespace,
			v1.SchemeGroupVersion.WithResource("elasticsearchmetricsets"),
			v1.SchemeGroupVersion.WithKind("ElasticsearchMetricSet"),
			func() *v1.ElasticsearchMetricSet { return &v1.ElasticsearchMetricSet{} },
			func() *v1.ElasticsearchMetricSetList { return &v1.ElasticsearchMetricSetList{} },
			func(dst, src *v1.ElasticsearchMetricSetList) { dst.ListMeta = src.ListMeta },
			func(list *v1.ElasticsearchMetricSetList) []*v1.ElasticsearchMetricSet {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1.ElasticsearchMetricSetList, items []*v1.ElasticsearchMetricSet) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
