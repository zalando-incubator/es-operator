/*
Copyright 2023 The Kubernetes Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	zalandoorgv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	versioned "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned"
	internalinterfaces "github.com/zalando-incubator/es-operator/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/zalando-incubator/es-operator/pkg/client/listers/zalando.org/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ElasticsearchMetricSetInformer provides access to a shared informer and lister for
// ElasticsearchMetricSets.
type ElasticsearchMetricSetInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.ElasticsearchMetricSetLister
}

type elasticsearchMetricSetInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewElasticsearchMetricSetInformer constructs a new informer for ElasticsearchMetricSet type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewElasticsearchMetricSetInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredElasticsearchMetricSetInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredElasticsearchMetricSetInformer constructs a new informer for ElasticsearchMetricSet type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredElasticsearchMetricSetInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ZalandoV1().ElasticsearchMetricSets(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ZalandoV1().ElasticsearchMetricSets(namespace).Watch(context.TODO(), options)
			},
		},
		&zalandoorgv1.ElasticsearchMetricSet{},
		resyncPeriod,
		indexers,
	)
}

func (f *elasticsearchMetricSetInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredElasticsearchMetricSetInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *elasticsearchMetricSetInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&zalandoorgv1.ElasticsearchMetricSet{}, f.defaultInformer)
}

func (f *elasticsearchMetricSetInformer) Lister() v1.ElasticsearchMetricSetLister {
	return v1.NewElasticsearchMetricSetLister(f.Informer().GetIndexer())
}
