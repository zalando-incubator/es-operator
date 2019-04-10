package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestEDSSpecFactory struct {
	edsName  string
	replicas int32
	scaling  *zv1.ElasticsearchDataSetScaling
}

func NewTestEDSSpecFactory(edsName string) *TestEDSSpecFactory {
	return &TestEDSSpecFactory{
		edsName:  edsName,
		replicas: 1,
	}
}

func (f *TestEDSSpecFactory) Replicas(replicas int32) *TestEDSSpecFactory {
	f.replicas = replicas
	return f
}

func (f *TestEDSSpecFactory) Scaling(scaling *zv1.ElasticsearchDataSetScaling) *TestEDSSpecFactory {
	f.scaling = scaling
	return f
}

func (f *TestEDSSpecFactory) Create() zv1.ElasticsearchDataSetSpec {
	var result = zv1.ElasticsearchDataSetSpec{
		Replicas: &f.replicas,
		Scaling:  f.scaling,
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "es-operator",
					"component":   "elasticsearch",
				},
			},
			Spec: edsPodSpec(f.edsName),
		},
	}

	return result
}

func testEDSCreate(t *testing.T, edsName string) zv1.ElasticsearchDataSetSpec {
	edsSpecFactory := NewTestEDSSpecFactory(edsName)
	edsSpec := edsSpecFactory.Create()

	err := createEDS(edsName, edsSpec)
	require.NoError(t, err)
	return edsSpec
}

func verifyEDS(t *testing.T, edsName string, edsSpec zv1.ElasticsearchDataSetSpec, replicas *int32) *zv1.ElasticsearchDataSet {
	// Verify eds
	eds, err := waitForEDS(t, edsName)
	require.NoError(t, err)
	err = waitForEDSCondition(t, eds.Name, func(eds *zv1.ElasticsearchDataSet) error {
		if !assert.ObjectsAreEqualValues(replicas, eds.Spec.Replicas) {
			return fmt.Errorf("%s: replicas %d != expected %d", eds.Name, *eds.Spec.Replicas, *replicas)
		}
		if !assert.ObjectsAreEqualValues(edsSpec.Template.Labels, eds.Spec.Template.Labels) {
			return fmt.Errorf("EDS '%s' labels %v != expected %v", eds.Name, eds.Spec.Template.Labels, edsSpec.Template.Labels)
		}
		return nil
	})
	require.NoError(t, err)

	// Verify statefulset
	sts, err := waitForStatefulSet(t, edsName)
	require.NoError(t, err)
	require.EqualValues(t, replicas, sts.Spec.Replicas)

	// Verify service
	service, err := waitForService(t, eds.Name)
	require.NoError(t, err)
	require.EqualValues(t, eds.Labels, service.Labels)
	require.EqualValues(t, sts.Spec.Selector.MatchLabels, service.Spec.Selector)

	// wait for this condition to be true
	err = waitForSTSCondition(t, sts.Name, func(sts *appsv1.StatefulSet) error {
		if !assert.ObjectsAreEqualValues(mergeLabels(edsSpec.Template.Labels, sts.Spec.Selector.MatchLabels), sts.Spec.Template.Labels) {
			return fmt.Errorf("EDS '%s' labels %v, does not match STS labels %v", eds.Name, mergeLabels(edsSpec.Template.Labels, sts.Spec.Selector.MatchLabels), sts.Spec.Template.Labels)
		}
		return nil
	},
		expectedStsStatus{
			replicas:        replicas,
			updatedReplicas: replicas,
			readyReplicas:   replicas,
		}.matches)
	require.NoError(t, err)
	return eds
}

func mergeLabels(labelsSlice ...map[string]string) map[string]string {
	newLabels := make(map[string]string)
	for _, labels := range labelsSlice {
		for k, v := range labels {
			newLabels[k] = v
		}
	}
	return newLabels
}

func TestEDSCreateBasic(t *testing.T) {
	t.Parallel()
	edsName := "basic"
	edsSpec := testEDSCreate(t, edsName)
	verifyEDS(t, edsName, edsSpec, edsSpec.Replicas)
	deleteEDS(edsName)
}
