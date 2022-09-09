package operator

import (
	"context"
	"testing"
	"time"

	"github.com/zalando-incubator/es-operator/pkg/clientset"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	fakeclientset "github.com/zalando-incubator/es-operator/pkg/client/clientset/versioned/fake"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakemetrics "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestHasOwnership(t *testing.T) {
	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				esOperatorAnnotationKey: "my-operator",
			},
		},
	}

	operator := &ElasticsearchOperator{
		operatorID: "my-operator",
	}
	assert.True(t, operator.hasOwnership(eds))

	eds.Annotations[esOperatorAnnotationKey] = "not-my-operator"
	assert.False(t, operator.hasOwnership(eds))

	delete(eds.Annotations, esOperatorAnnotationKey)
	assert.False(t, operator.hasOwnership(eds))

	operator.operatorID = ""
	assert.True(t, operator.hasOwnership(eds))
}

func TestGetElasticsearchEndpoint(t *testing.T) {
	faker := &clientset.Clientset{
		Interface: fake.NewSimpleClientset(),
	}
	esOperator := NewElasticsearchOperator(faker, nil, 1*time.Second, 1*time.Second, "", "", "cluster.local.", nil)

	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}

	url := esOperator.getElasticsearchEndpoint(eds)
	assert.Equal(t, "http://foo.bar.svc.cluster.local.:9200", url.String())

	customURL := "http://127.0.0.1:8001/api/v1/namespaces/default/services/elasticsearch:9200/proxy"
	customEndpoint, err := url.Parse(customURL)
	assert.NoError(t, err)

	esOperator = NewElasticsearchOperator(faker, nil, 1*time.Second, 1*time.Second, "", "", ".cluster.local.", customEndpoint)
	url = esOperator.getElasticsearchEndpoint(eds)
	assert.Equal(t, customURL, url.String())
}

func TestGetOwnerUID(t *testing.T) {
	objectMeta := metav1.ObjectMeta{
		OwnerReferences: []metav1.OwnerReference{
			{
				UID: types.UID("x"),
			},
		},
	}

	uid, ok := getOwnerUID(objectMeta)
	assert.Equal(t, types.UID("x"), uid)
	assert.True(t, ok)

	uid, ok = getOwnerUID(metav1.ObjectMeta{})
	assert.Equal(t, types.UID(""), uid)
	assert.False(t, ok)
}

func TestTemplateInjectLabels(t *testing.T) {
	template := v1.PodTemplateSpec{}
	labels := map[string]string{"foo": "bar"}

	expectedTemplate := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
	}

	newTemplate := templateInjectLabels(template, labels)
	assert.Equal(t, expectedTemplate, newTemplate)
}

func TestValidateScalingSettings(tt *testing.T) {
	for _, tc := range []struct {
		msg     string
		scaling *zv1.ElasticsearchDataSetScaling
		err     bool
	}{
		{
			msg: "test simple valid scaling config",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled:          true,
				MinReplicas:      1,
				MaxReplicas:      3,
				MinIndexReplicas: 0,
				MaxIndexReplicas: 2,
				MinShardsPerNode: 1,
				MaxShardsPerNode: 1,
			},
		},
		{
			msg: "test min > max replicas",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled:          true,
				MinReplicas:      2,
				MaxReplicas:      1,
				MinIndexReplicas: 0,
				MaxIndexReplicas: 2,
				MinShardsPerNode: 1,
				MaxShardsPerNode: 1,
			},
			err: true,
		},
		{
			msg: "test min > max indexReplicas",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled:          true,
				MinReplicas:      1,
				MaxReplicas:      1,
				MinIndexReplicas: 2,
				MaxIndexReplicas: 1,
				MinShardsPerNode: 1,
				MaxShardsPerNode: 1,
			},
			err: true,
		},
		{
			msg: "test min > max shardsPerNode",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled:          true,
				MinReplicas:      1,
				MaxReplicas:      1,
				MinIndexReplicas: 1,
				MaxIndexReplicas: 1,
				MinShardsPerNode: 2,
				MaxShardsPerNode: 1,
			},
			err: true,
		},
		{
			msg: "test min possible shardsPerNode lower than minShardsPerNode",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled:          true,
				MinReplicas:      1,
				MaxReplicas:      2,
				MinIndexReplicas: 0,
				MaxIndexReplicas: 3,
				MinShardsPerNode: 2,
				MaxShardsPerNode: 2,
			},
			err: true,
		},
		{
			msg: "test minShardsPerNode > 0 and minReplicas < 1",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled:          true,
				MinReplicas:      0,
				MaxReplicas:      2,
				MinIndexReplicas: 1,
				MaxIndexReplicas: 3,
				MinShardsPerNode: 1,
				MaxShardsPerNode: 2,
			},
			err: true,
		},
		{
			msg: "scaling disabled",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled: false,
			},
		},
	} {
		tt.Run(tc.msg, func(t *testing.T) {
			err := validateScalingSettings(tc.scaling)
			if !tc.err {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func mockEDSResource(eds *zv1.ElasticsearchDataSet) EDSResource {
	fakeKube := &clientset.Clientset{
		Interface:  fake.NewSimpleClientset(),
		ZInterface: fakeclientset.NewSimpleClientset(),
		MInterface: fakemetrics.NewSimpleClientset(),
	}
	return EDSResource{
		eds:      eds,
		kube:     fakeKube,
		esClient: &ESClient{},
		recorder: nil,
	}
}

func TestNoStatusUpdateMisMatchedGeneration(t *testing.T) {
	stsReplicas := int32(2)
	sts := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &stsReplicas,
		},
	}
	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 0,
		},
		Status: zv1.ElasticsearchDataSetStatus{
			Replicas: stsReplicas,
		},
	}
	edsResource := mockEDSResource(eds)

	err := edsResource.UpdateStatus(context.Background(), sts)
	assert.Nil(t, err)
	assert.Nil(t, eds.Status.ObservedGeneration)
	assert.Equal(t, stsReplicas, eds.Status.Replicas)
	assert.Equal(t, int32(0), eds.Status.HpaReplicas)
	assert.Equal(t, "", eds.Status.Selector)
	assert.Equal(t, "", eds.TypeMeta.Kind)
	assert.Equal(t, "", eds.TypeMeta.APIVersion)
}

func TestNoStatusUpdateEqualSTSReplicas(t *testing.T) {
	stsReplicas := int32(2)
	sts := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &stsReplicas,
		},
	}
	observedGeneration := int64(0)
	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Generation: observedGeneration,
		},
		Status: zv1.ElasticsearchDataSetStatus{
			Replicas:           stsReplicas,
			ObservedGeneration: &observedGeneration,
		},
	}
	edsResource := mockEDSResource(eds)

	err := edsResource.UpdateStatus(context.Background(), sts)
	assert.Nil(t, err)
	assert.Equal(t, &observedGeneration, eds.Status.ObservedGeneration)
	assert.Equal(t, stsReplicas, eds.Status.Replicas)
	assert.Equal(t, int32(0), eds.Status.HpaReplicas)
	assert.Equal(t, "", eds.Status.Selector)
	assert.Equal(t, "", eds.TypeMeta.Kind)
	assert.Equal(t, "", eds.TypeMeta.APIVersion)
}

func TestSuccessfulUpdateStatus(t *testing.T) {
	stsReplicas := int32(2)
	sts := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &stsReplicas,
		},
	}
	observedGeneration := int64(0)
	statusReplicas := int32(4)
	objectName := "es-data-test"
	fakeKube := &clientset.Clientset{
		Interface:  fake.NewSimpleClientset(),
		ZInterface: fakeclientset.NewSimpleClientset(),
		MInterface: fakemetrics.NewSimpleClientset(),
	}
	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       objectName,
			Generation: observedGeneration,
			Namespace:  "test",
		},
		Status: zv1.ElasticsearchDataSetStatus{
			Replicas:           statusReplicas,
			ObservedGeneration: &observedGeneration,
		},
	}
	createdEds, err := fakeKube.ZalandoV1().ElasticsearchDataSets(eds.Namespace).Create(context.Background(), eds, metav1.CreateOptions{})
	assert.Nil(t, err)

	edsResource := EDSResource{
		eds:      createdEds,
		kube:     fakeKube,
		esClient: &ESClient{},
		recorder: nil,
	}

	err = edsResource.UpdateStatus(context.Background(), sts)
	assert.Nil(t, err)
	assert.Equal(t, &observedGeneration, createdEds.Status.ObservedGeneration)
	assert.Equal(t, stsReplicas, createdEds.Status.Replicas)
	assert.Equal(t, int32(1), createdEds.Status.HpaReplicas)
	assert.Equal(t, objectName, createdEds.Status.Selector)
	assert.Equal(t, "", createdEds.TypeMeta.Kind)
	assert.Equal(t, "", createdEds.TypeMeta.APIVersion)
}
