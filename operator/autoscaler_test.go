package operator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScalingDirection(t *testing.T) {
	eds := &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "EDS1",
			Namespace: "default",
		},
		Spec: zv1.ElasticsearchDataSetSpec{
			Scaling: &zv1.ElasticsearchDataSetScaling{
				ScaleUpCooldownSeconds:            120,
				ScaleUpCPUBoundary:                50,
				ScaleUpThresholdDurationSeconds:   240,
				ScaleDownCooldownSeconds:          120,
				ScaleDownCPUBoundary:              25,
				ScaleDownThresholdDurationSeconds: 240,
			},
		},
		Status: zv1.ElasticsearchDataSetStatus{},
	}
	esMSet := &zv1.ElasticsearchMetricSet{
		Metrics: []zv1.ElasticsearchMetric{
			{
				Timestamp: metav1.Now(),
				Value:     20,
			},
		},
	}

	as := systemUnderTest(eds, esMSet, nil)

	// don't scale: not enough samples.
	require.Equal(t, NONE, as.getScalingDirection())

	esMSet.Metrics = []zv1.ElasticsearchMetric{
		{
			Timestamp: metav1.Now(),
			Value:     20,
		},
		{
			Timestamp: metav1.Now(),
			Value:     20,
		},
		{
			Timestamp: metav1.Now(),
			Value:     20,
		},
		{
			Timestamp: metav1.Now(),
			Value:     20,
		},
	}

	// scale down
	require.Equal(t, DOWN, as.getScalingDirection())

	esMSet.Metrics = []zv1.ElasticsearchMetric{
		{
			Timestamp: metav1.Now(),
			Value:     20,
		},
		{
			Timestamp: metav1.Now(),
			Value:     20,
		},
		{
			Timestamp: metav1.Now(),
			Value:     20,
		},
		{
			Timestamp: metav1.Now(),
			Value:     30,
		},
	}

	// don't scale: one sample not in threshold
	require.Equal(t, NONE, as.getScalingDirection())

	esMSet.Metrics = []zv1.ElasticsearchMetric{
		{
			Timestamp: metav1.Now(),
			Value:     25,
		},
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
	}

	// scale up
	require.Equal(t, UP, as.getScalingDirection())

	esMSet.Metrics = []zv1.ElasticsearchMetric{
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
		{
			Timestamp: metav1.Now(),
			Value:     55,
		},
	}

	now := metav1.Now()
	eds.Status.LastScaleUpStarted = &now

	// don't scale: cool-down period.
	require.Equal(t, NONE, as.getScalingDirection())

}

func TestScaleUp(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// scale up
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 3, Primaries: 6, Index: "ad1"},
	}
	direction := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(5), *actual.NodeReplicas)
}

func TestScaleUpByAddingReplicas(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// scale up by adding replicas
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 4, Index: "ad1"},
	}
	direction := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(6), *actual.NodeReplicas)
	require.Equal(t, int32(2), actual.IndexReplicas[0].Replicas)
}

func TestScaleDown(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// scale down
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 6, Index: "ad1"},
	}
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(3), *actual.NodeReplicas)
}

func TestCannotScaleDownAnymore(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// cannot scale down anymore
	eds.Spec.Replicas = &desiredReplicas
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 6, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 6, Index: "ad2"},
	}
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Nil(t, actual.NodeReplicas)
}

func TestIncreaseShardToNodeRatioMore(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// scale-down even if this means increasing shard-to-node ratio of more than +1
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 0, Primaries: 21, Index: "ad1"},
		"ad2": {Replicas: 0, Primaries: 2, Index: "ad2"},
	}
	newDesired := int32(3)
	eds.Spec.Replicas = &newDesired
	eds.Spec.Scaling.MaxShardsPerNode = 23
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(2), *actual.NodeReplicas)
}

func TestScaleDownByRemovingIndexReplica(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// scale down by removing an index replica
	eds.Spec.Replicas = &desiredReplicas
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 2, Primaries: 4, Index: "ad1"},
	}
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(3), *actual.NodeReplicas)
	require.Equal(t, int32(1), actual.IndexReplicas[0].Replicas)
}

func TestAtMaxIndexReplicas(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// cannot scale up further (already at maxIndexReplicas)
	eds.Spec.Replicas = &desiredReplicas
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 3, Primaries: 2, Index: "ad1"},
	}
	direction := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas))

}

func TestScaleUpCausedByShardToNodeRatioExceeded(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// require to scale-up because we exceeded shard-to-node ratio limits.
	eds.Spec.Replicas = &desiredReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 6
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 5, Primaries: 10, Index: "ad1"},
	}
	// scaling independent of desired scaling direction
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(10), *actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas))
}

func TestAtMaxShardsPerNode(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// don't scale down if that would violate the maxShardsPerNode
	eds.Spec.Replicas = &desiredReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 6
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 9, Index: "ad1"},
		"ad2": {Replicas: 0, Primaries: 5, Index: "ad2"},
	}
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas))
}

func TestAtMinIndexReplicas(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// don't scale down if that would cause to have less data nodes then min index replicas
	eds.Spec.Replicas = &desiredReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 24
	eds.Spec.Scaling.MinIndexReplicas = 3

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 3, Primaries: 1, Index: "ad1"},
		"ad2": {Replicas: 3, Primaries: 1, Index: "ad2"},
	}
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas))
}

func TestAtNoIndicesAllocatedYet(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esIndices := make(map[string]ESIndex)
	esNodes := make([]ESNode, 0)

	// don't scale down if no indices are allocated yet
	eds.Spec.Replicas = &desiredReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 24
	eds.Spec.Scaling.MinIndexReplicas = 0

	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas))
}

func TestAtMinReplicas(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// don't scale down if we reached MinReplicas
	eds.Spec.Replicas = &desiredReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 24
	eds.Spec.Scaling.MinIndexReplicas = 1
	eds.Spec.Scaling.MinReplicas = 4

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 1, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 1, Index: "ad2"},
	}
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(4), *actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas))
}

func TestAtMaxReplicas(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := make([]ESNode, 0)

	// don't scale down if we reached MinReplicas
	eds.Spec.Replicas = &desiredReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 2
	eds.Spec.Scaling.MinIndexReplicas = 1
	eds.Spec.Scaling.MaxReplicas = 4

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 3, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 3, Index: "ad2"},
	}
	direction := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(4), *actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas))
}

func TestAtMaxDisk(t *testing.T) {
	desiredReplicas := int32(4)
	eds := edsTestFixture(desiredReplicas)
	esNodes := []ESNode{
		{
			IP:              "1.2.3.4",
			DiskUsedPercent: 60.0,
		},
		{
			IP:              "1.2.3.4",
			DiskUsedPercent: 80.0,
		},
	}

	// don't scale down if we reached MaxDisk
	eds.Spec.Replicas = &desiredReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 12
	eds.Spec.Scaling.MinIndexReplicas = 1
	eds.Spec.Scaling.MaxReplicas = 2
	eds.Spec.Scaling.MinReplicas = 1

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 6, Index: "ad1"},
	}
	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, actual.ScalingDirection, NONE)
}

func edsTestFixture(desiredReplicas int32) *zv1.ElasticsearchDataSet {
	return &zv1.ElasticsearchDataSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "EDS1",
			Namespace: "default",
		},
		Spec: zv1.ElasticsearchDataSetSpec{
			Scaling: &zv1.ElasticsearchDataSetScaling{
				MinShardsPerNode:                   2,
				MaxShardsPerNode:                   6,
				MinIndexReplicas:                   1,
				MaxIndexReplicas:                   3,
				DiskUsagePercentScaledownWatermark: 75.0,
			},
			Replicas: &desiredReplicas,
		},
	}
}

func TestGetManagedIndices(t *testing.T) {
	pods := []v1.Pod{
		{
			Status: v1.PodStatus{
				PodIP: "1.2.3.4",
			},
		},
		{
			Status: v1.PodStatus{
				PodIP: "1.2.3.5",
			},
		},
	}

	indices := []ESIndex{
		{
			Index:     "a",
			Replicas:  2,
			Primaries: 1,
		},
		{
			Index:     "b",
			Replicas:  1,
			Primaries: 1,
		},
		{
			Index:     "c",
			Replicas:  1,
			Primaries: 1,
		},
	}

	shards := []ESShard{
		{
			Index: "a",
			IP:    "1.2.3.4",
		},
		{
			Index: "a",
			IP:    "1.2.3.5",
		},
		{
			Index: "b",
			IP:    "1.2.4.5",
		},
		{
			Index: "c",
			IP:    "1.2.3.4",
		},
	}

	as := systemUnderTest(edsTestFixture(1), nil, pods)
	actual := as.getManagedIndices(indices, shards)

	require.Equal(t, 2, len(actual))
	require.Equal(t, "a", actual["a"].Index)
	require.Equal(t, "c", actual["c"].Index)
}

func systemUnderTest(eds *zv1.ElasticsearchDataSet, metricSet *zv1.ElasticsearchMetricSet, pods []v1.Pod) *AutoScaler {
	es := &ESResource{
		ElasticsearchDataSet: eds,
		MetricSet:            metricSet,
		Pods:                 pods,
	}
	return NewAutoScaler(es, time.Second*60, nil)
}
