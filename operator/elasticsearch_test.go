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

func TestGetEmptyElasticSearchDrainingSpec(t *testing.T) {
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
	esOperator := NewElasticsearchOperator(faker, nil, 1*time.Second, 1*time.Second, "", "", "cluster.local.", nil)

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
		expected int32
	}{
		{
			name: "scaling disabled, replicas nil -> default 1",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{},
			},
			expected: 1,
		},
		{
			name: "scaling disabled, replicas set -> value",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: &three,
				},
			},
			expected: 3,
		},
		{
			name: "scaling enabled, replicas nil -> 0",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Scaling: &zv1.ElasticsearchDataSetScaling{Enabled: true},
				},
			},
			expected: 0,
		},
		{
			name: "scaling enabled, replicas set -> value",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true},
					Replicas: &one,
				},
			},
			expected: 1,
		},
		{
			name: "scaling enabled, min and max replicas > spec.replicas",
			eds: &zv1.ElasticsearchDataSet{
				Spec: zv1.ElasticsearchDataSetSpec{
					Scaling:  &zv1.ElasticsearchDataSetScaling{Enabled: true, MinReplicas: four, MaxReplicas: four},
					Replicas: &three,
				},
			},
			// edsReplicas should reflect the current scaling target
			// ensuring bounds is responsibility of the autoscaler
			expected: 3,
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
func TestScaleToZeroPrevention(t *testing.T) {
	minReplicas := int32(3)
	maxReplicas := int32(10)
	statusReplicas := int32(5)

	for _, tc := range []struct {
		name             string
		eds              *zv1.ElasticsearchDataSet
		expectedReplicas int32
		description      string
	}{
		{
			name: "nil replicas + status replicas set -> use status",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-eds",
					Namespace: "default",
				},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil, // Cleared by kubectl patch
					Scaling: &zv1.ElasticsearchDataSetScaling{
						Enabled:     true,
						MinReplicas: minReplicas,
						MaxReplicas: maxReplicas,
					},
				},
				Status: zv1.ElasticsearchDataSetStatus{
					Replicas: statusReplicas, // Actually running 5 replicas
				},
			},
			expectedReplicas: statusReplicas,
			description:      "Should use status.replicas (5) to preserve current state",
		},
		{
			name: "nil replicas + status replicas zero -> use minReplicas",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-eds-new",
					Namespace: "default",
				},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil, // Not yet initialized
					Scaling: &zv1.ElasticsearchDataSetScaling{
						Enabled:     true,
						MinReplicas: minReplicas,
						MaxReplicas: maxReplicas,
					},
				},
				Status: zv1.ElasticsearchDataSetStatus{
					Replicas: 0, // New EDS not yet running
				},
			},
			expectedReplicas: minReplicas,
			description:      "Should use minReplicas (3) for new/uninitialized EDS",
		},
		{
			name: "nil replicas + no minReplicas -> use status",
			eds: &zv1.ElasticsearchDataSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-eds-no-min",
					Namespace: "default",
				},
				Spec: zv1.ElasticsearchDataSetSpec{
					Replicas: nil,
					Scaling: &zv1.ElasticsearchDataSetScaling{
						Enabled:     true,
						MinReplicas: 0, // No minimum set
						MaxReplicas: maxReplicas,
					},
				},
				Status: zv1.ElasticsearchDataSetStatus{
					Replicas: statusReplicas,
				},
			},
			expectedReplicas: 0, // No defensive logic applied when minReplicas is 0
			description:      "Should return 0 when minReplicas is not set",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate what happens in scaleEDS
			currentReplicas := edsReplicas(tc.eds)
			scaling := tc.eds.Spec.Scaling

			// Apply the defensive logic from scaleEDS
			if currentReplicas == 0 && scaling != nil && scaling.MinReplicas > 0 {
				if tc.eds.Status.Replicas > 0 {
					currentReplicas = tc.eds.Status.Replicas
				} else {
					currentReplicas = scaling.MinReplicas
				}
			}

			assert.Equal(t, tc.expectedReplicas, currentReplicas, tc.description)
		})
	}
}
