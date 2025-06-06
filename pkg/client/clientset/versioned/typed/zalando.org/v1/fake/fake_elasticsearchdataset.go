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

// fakeElasticsearchDataSets implements ElasticsearchDataSetInterface
type fakeElasticsearchDataSets struct {
	*gentype.FakeClientWithList[*v1.ElasticsearchDataSet, *v1.ElasticsearchDataSetList]
	Fake *FakeZalandoV1
}

func newFakeElasticsearchDataSets(fake *FakeZalandoV1, namespace string) zalandoorgv1.ElasticsearchDataSetInterface {
	return &fakeElasticsearchDataSets{
		gentype.NewFakeClientWithList[*v1.ElasticsearchDataSet, *v1.ElasticsearchDataSetList](
			fake.Fake,
			namespace,
			v1.SchemeGroupVersion.WithResource("elasticsearchdatasets"),
			v1.SchemeGroupVersion.WithKind("ElasticsearchDataSet"),
			func() *v1.ElasticsearchDataSet { return &v1.ElasticsearchDataSet{} },
			func() *v1.ElasticsearchDataSetList { return &v1.ElasticsearchDataSetList{} },
			func(dst, src *v1.ElasticsearchDataSetList) { dst.ListMeta = src.ListMeta },
			func(list *v1.ElasticsearchDataSetList) []*v1.ElasticsearchDataSet {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1.ElasticsearchDataSetList, items []*v1.ElasticsearchDataSet) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
