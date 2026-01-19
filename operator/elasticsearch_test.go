package operator

import (
	"testing"
	"time"

	"github.com/zalando-incubator/es-operator/pkg/clientset"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	esOperator := NewElasticsearchOperator(faker, nil, 1*time.Second, 1*time.Second, "", "", "cluster.local.", nil, false)

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

	esOperator = NewElasticsearchOperator(faker, nil, 1*time.Second, 1*time.Second, "", "", ".cluster.local.", customEndpoint, false)
	url = esOperator.getElasticsearchEndpoint(eds)
	assert.Equal(t, customURL, url.String())
}

func TestGetEmptyElasticSearchDrainingSpec(t *testing.T) {
	faker := &clientset.Clientset{
		Interface: fake.NewSimpleClientset(),
	}
	esOperator := NewElasticsearchOperator(faker, nil, 1*time.Second, 1*time.Second, "", "", "cluster.local.", nil, false)

	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}

	config := esOperator.getDrainingConfig(eds)
	assert.NotNil(t, config)
	assert.Equal(t, config.MaxRetries, 999)
	assert.Equal(t, config.MinimumWaitTime, 10*time.Second)
	assert.Equal(t, config.MaximumWaitTime, 30*time.Second)
}

func TestGetNotEmptyElasticSearchDrainingSpec(t *testing.T) {
	faker := &clientset.Clientset{
		Interface: fake.NewSimpleClientset(),
	}
	esOperator := NewElasticsearchOperator(faker, nil, 1*time.Second, 1*time.Second, "", "", "cluster.local.", nil, false)

	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: zv1.ElasticsearchDataSetSpec{
			Experimental: &zv1.ExperimentalSpec{
				Draining: &zv1.ElasticsearchDataSetDraining{
					MaxRetries:                     7,
					MinimumWaitTimeDurationSeconds: 2,
					MaximumWaitTimeDurationSeconds: 34,
				},
			},
		},
	}

	config := esOperator.getDrainingConfig(eds)
	assert.NotNil(t, config)
	assert.Equal(t, config.MaxRetries, 7)
	assert.Equal(t, config.MinimumWaitTime, 2*time.Second)
	assert.Equal(t, config.MaximumWaitTime, 34*time.Second)
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
			msg: "test minReplicas = 0 with autoscaling enabled (scale-to-zero prevention)",
			scaling: &zv1.ElasticsearchDataSetScaling{
				Enabled:          true,
				MinReplicas:      0,
				MaxReplicas:      10,
				MinIndexReplicas: 0,
				MaxIndexReplicas: 2,
				MinShardsPerNode: 1,
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

func TestEDSReplicas(t *testing.T) {
	one := int32(1)
	three := int32(3)
	four := int32(4)

	for _, tc := range []struct {
		name     string
		eds      *zv1.ElasticsearchDataSet
		expected *int32
	}{
		{
			name: "scaling disabled, replicas nil -> nil",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{},
			},
			expected: nil,
		},
		{
			name: "scaling disabled, replicas set -> value",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: &three,
				},
			},
			expected: &three,
		},
		{
			name: "scaling enabled, replicas nil + min=0 -> nil",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Scaling: &zv1.ElasticsearchDataSetScaling{Enabled: true},
				},
			},
			expected: nil,
		},
		{
			name: "scaling enabled, replicas set -> value",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true},
					Replicas: &one,
				},
			},
			expected: &one,
		},
		{
			name: "scaling enabled, min and max replicas > spec.replicas",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true, MinReplicas: four, MaxReplicas: four},
					Replicas: &three,
				},
			},
			// edsReplicas should reflect the current scaling target.
			// Ensuring bounds is responsibility of the autoscaler.
			expected: &three,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := edsReplicas(tc.eds)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

// TestScaleToZeroPrevention validates that the defensive logic in scaleEDS prevents
// writing 0 to spec.replicas when it would violate minReplicas.
// This test documents the fix for the regression introduced in PR #511 where:
//   - kubectl patch operations could leave spec.replicas as nil
//   - edsReplicas would return 0 for autoscaling-enabled EDS
//   - When autoscaler returned no-op (e.g., excludeSystemIndices filters all indices),
//     spec.replicas would be written as 0, violating minReplicas
func TestEDSReplicasFallback(t *testing.T) {
	minReplicas := int32(3)
	maxReplicas := int32(10)
	statusReplicas := int32(5)

	for _, tc := range []struct {
		name        string
		eds         *zv1.ElasticsearchDataSet
		expected    *int32
		description string
	}{
		{
			name: "autoscaling enabled, nil replicas + status set -> max(status,min)",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-eds", Namespace: "default"},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil,
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true, MinReplicas: minReplicas, MaxReplicas: maxReplicas},
				},
				Status: zv1.ElasticsearchDataSetStatus{Replicas: statusReplicas},
			},
			expected:    &statusReplicas,
			description: "Preserve current running replicas and enforce minReplicas",
		},
		{
			name: "autoscaling enabled, nil replicas + status zero -> minReplicas",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-eds-new", Namespace: "default"},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil,
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true, MinReplicas: minReplicas, MaxReplicas: maxReplicas},
				},
				Status: zv1.ElasticsearchDataSetStatus{Replicas: 0},
			},
			expected:    &minReplicas,
			description: "Initialize to minReplicas for new clusters",
		},
		{
			name: "autoscaling enabled, nil replicas + minReplicas zero -> nil",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-eds-no-min", Namespace: "default"},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil,
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true, MinReplicas: 0, MaxReplicas: maxReplicas},
				},
				Status: zv1.ElasticsearchDataSetStatus{Replicas: statusReplicas},
			},
			expected:    nil,
			description: "Note: minReplicas=0 with autoscaling enabled should be rejected by validation, but edsReplicas returns nil when it occurs",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := edsReplicas(tc.eds)
			assert.Equal(t, tc.expected, actual, tc.description)
		})
	}
}

// TestEDSReplicasEnforcesMinimumOne verifies that edsReplicas enforces
// a minimum of 1 replica to prevent scale-to-zero scenarios.
func TestEDSReplicasEnforcesMinimumOne(t *testing.T) {
	one := int32(1)
	minReplicas := int32(1)
	maxReplicas := int32(10)

	for _, tc := range []struct {
		name     string
		eds      *zv1.ElasticsearchDataSet
		expected *int32
	}{
		{
			name: "minReplicas=1, status=0 -> returns 1",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-eds", Namespace: "default"},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil,
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true, MinReplicas: minReplicas, MaxReplicas: maxReplicas},
				},
				Status: zv1.ElasticsearchDataSetStatus{Replicas: 0},
			},
			expected: &one,
		},
		{
			name: "minReplicas=1, status=5 -> returns 5",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-eds", Namespace: "default"},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil,
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true, MinReplicas: minReplicas, MaxReplicas: maxReplicas},
				},
				Status: zv1.ElasticsearchDataSetStatus{Replicas: 5},
			},
			expected: &[]int32{5}[0],
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := edsReplicas(tc.eds)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
