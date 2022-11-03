package operator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScalingHint(t *testing.T) {
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
	require.Equal(t, NONE, as.scalingHint())

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
	require.Equal(t, DOWN, as.scalingHint())

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
	require.Equal(t, NONE, as.scalingHint())

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
	require.Equal(t, UP, as.scalingHint())

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
	require.Equal(t, NONE, as.scalingHint())

}

func TestScaleUp(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// scale up
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 3, Primaries: 6, Index: "ad1"},
	}
	scalingHint := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(5), *actual.NodeReplicas, actual.Description)
	require.Equal(t, UP, actual.ScalingDirection, actual.Description)
}

func TestScaleUpByAddingReplicas(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// scale up by adding replicas
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 4, Index: "ad1"},
	}
	scalingHint := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(6), *actual.NodeReplicas, actual.Description)
	require.Equal(t, int32(2), actual.IndexReplicas[0].Replicas, actual.Description)
	require.Equal(t, UP, actual.ScalingDirection, actual.Description)
}

func TestScaleUpSnappingToNonFractionedShardToNodeRatio(t *testing.T) {
	eds := edsTestFixture(5)
	esNodes := make([]ESNode, 0)

	// scale up by adding replicas
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 4, Index: "ad1"},
	}
	eds.Spec.Scaling.MinShardsPerNode = 1
	direction := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(8), *actual.NodeReplicas, actual.Description)
}

func TestScaleDownSnappingToNonFractionedShardToNodeRatio(t *testing.T) {
	eds := edsTestFixture(14)
	esNodes := make([]ESNode, 0)

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 12, Index: "ad1"},
	}
	eds.Spec.Scaling.MinShardsPerNode = 1

	direction := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, direction)
	require.Equal(t, int32(12), *actual.NodeReplicas, actual.Description)
}

func TestScaleDown(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// scale down
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 6, Index: "ad1"},
	}
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(3), *actual.NodeReplicas, actual.Description)
	require.Equal(t, DOWN, actual.ScalingDirection, actual.Description)
}

func TestScaleDownToLowestBoundary(t *testing.T) {
	eds := edsTestFixture(4)
	eds.Spec.Scaling.MaxShardsPerNode = 40

	esNodes := make([]ESNode, 0)

	// scale down
	esIndices := map[string]ESIndex{
		"a": {Replicas: 1, Primaries: 12, Index: "a"},
		"b": {Replicas: 1, Primaries: 12, Index: "b"},
		"c": {Replicas: 1, Primaries: 12, Index: "c"},
		"d": {Replicas: 1, Primaries: 12, Index: "d"},
		"e": {Replicas: 1, Primaries: 12, Index: "e"},
	}
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, DOWN, actual.ScalingDirection, actual.Description)
	require.Equal(t, int32(3), *actual.NodeReplicas, actual.Description)
}

func TestCannotScaleDownAnymore(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// cannot scale down anymore
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 6, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 6, Index: "ad2"},
	}
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Nil(t, actual.NodeReplicas, actual.Description)
	require.Equal(t, NONE, actual.ScalingDirection, actual.Description)
}

func TestIncreaseShardToNodeRatioMore(t *testing.T) {
	eds := edsTestFixture(3)
	esNodes := make([]ESNode, 0)

	// scale-down even if this means increasing shard-to-node ratio of more than +1
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 0, Primaries: 42, Index: "ad1"},
		"ad2": {Replicas: 0, Primaries: 4, Index: "ad2"},
	}
	eds.Spec.Scaling.MinIndexReplicas = 0
	eds.Spec.Scaling.MaxShardsPerNode = 23
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(2), *actual.NodeReplicas, actual.Description)
	require.Equal(t, DOWN, actual.ScalingDirection, actual.Description)
}

func TestScaleDownByRemovingIndexReplica(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// scale down by removing an index replica
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 2, Primaries: 4, Index: "ad1"},
	}
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(3), *actual.NodeReplicas, actual.Description)
	require.Equal(t, int32(1), actual.IndexReplicas[0].Replicas, actual.Description)
	require.Equal(t, DOWN, actual.ScalingDirection, actual.Description)
}

func TestAtMaxIndexReplicas(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// cannot scale up further (already at maxIndexReplicas)
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 3, Primaries: 2, Index: "ad1"},
	}
	scalingHint := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Nil(t, actual.NodeReplicas, actual.Description)
	require.Equal(t, 0, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, NONE, actual.ScalingDirection, actual.Description)
}

func TestScaleUpCausedByShardToNodeRatioExceeded(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// require to scale-up because we exceeded shard-to-node ratio limits.
	eds.Spec.Scaling.MaxShardsPerNode = 6
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 5, Primaries: 10, Index: "ad1"},
	}
	// irrelevant, because we're reconciling based on maxShardsPerNode setting
	scalingHint := NONE

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(10), *actual.NodeReplicas, actual.Description)
	require.Equal(t, 0, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, UP, actual.ScalingDirection, actual.Description)
}

func TestScaleUpCausedByShardToNodeRatioLessThanOne(t *testing.T) {
	eds := edsTestFixture(11)
	esNodes := make([]ESNode, 0)

	// require to scale-up index replicas because we are below one shard per node.
	eds.Spec.Scaling.MinReplicas = 11
	eds.Spec.Scaling.MaxReplicas = 999

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 5, Index: "ad1"},
	}
	// calculated ShardToNode ratio is 10/11 ~ 0.9
	eds.Spec.Scaling.MinShardsPerNode = 1
	// scaling independent of desired scaling direction
	scalingHint := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(11), *actual.NodeReplicas, actual.Description)
	require.Equal(t, 1, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, UP, actual.ScalingDirection, actual.Description)
}

func TestAtMaxShardsPerNode(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 18, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 5, Index: "ad2"},
	}
	// don't scale down if that would violate the maxShardsPerNode
	// calculations:
	//   ad1 shards = primaries shards + replicas shards = 18 + 1*18 = 36
	//   ad2 shards = 5 * 1*5 = 10
	//   total shards = 46
	//   per node = 46/4 = 11.5
	eds.Spec.Scaling.MaxShardsPerNode = 12

	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, NONE, actual.ScalingDirection, actual.Description)
}

func TestScaleUpIndexReplicasToMeetMinIndexReplicasBoundary(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// scale up indexReplicas to 2 despite scalingHint == DOWN
	eds.Spec.Scaling.MinIndexReplicas = 2
	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 0, Primaries: 1, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 1, Index: "ad2"},
		"ad3": {Replicas: 2, Primaries: 1, Index: "ad3"},
		"ad4": {Replicas: 3, Primaries: 1, Index: "ad4"},
	}
	// irrelevant, as we're reconciling based on the minIndexReplicas setting
	scalingHint := NONE

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, 2, len(actual.IndexReplicas), actual.Description)
	require.Contains(t, actual.IndexReplicas, ESIndex{Index: "ad1", Primaries: 1, Replicas: 2})
	require.Contains(t, actual.IndexReplicas, ESIndex{Index: "ad2", Primaries: 1, Replicas: 2})
	require.Equal(t, UP, actual.ScalingDirection, actual.Description)
}

func TestScaleDownIndexReplicasToStayWithinMaxIndexReplicasBoundary(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 4, Primaries: 1, Index: "ad1"},
		"ad2": {Replicas: 4, Primaries: 1, Index: "ad2"},
		"ad3": {Replicas: 3, Primaries: 1, Index: "ad3"},
		"ad4": {Replicas: 3, Primaries: 1, Index: "ad4"},
	}
	// irrelevant, as we're reconciling based on the maxIndexReplicas setting
	scalingHint := NONE

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, 2, len(actual.IndexReplicas), actual.Description)
	require.Contains(t, actual.IndexReplicas, ESIndex{Index: "ad1", Primaries: 1, Replicas: 3})
	require.Contains(t, actual.IndexReplicas, ESIndex{Index: "ad2", Primaries: 1, Replicas: 3})
	require.Equal(t, DOWN, actual.ScalingDirection, actual.Description)
}

func TestAtMinIndexReplicas(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// don't scale down if that would cause to have fewer data nodes than min index replicas
	eds.Spec.Scaling.MaxShardsPerNode = 24
	eds.Spec.Scaling.MinIndexReplicas = 3

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 3, Primaries: 1, Index: "ad1"},
		"ad2": {Replicas: 3, Primaries: 1, Index: "ad2"},
	}
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, NONE, actual.ScalingDirection, actual.Description)
}

func TestAtNoIndicesAllocatedYet(t *testing.T) {
	eds := edsTestFixture(4)
	esIndices := make(map[string]ESIndex)
	esNodes := make([]ESNode, 0)

	// don't scale down if no indices are allocated yet
	eds.Spec.Scaling.MaxShardsPerNode = 24
	eds.Spec.Scaling.MinIndexReplicas = 0

	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Nil(t, actual.NodeReplicas)
	require.Equal(t, 0, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, NONE, actual.ScalingDirection, actual.Description)
}

func TestAtMinReplicas(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// don't scale down if we reached MinReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 24
	eds.Spec.Scaling.MinIndexReplicas = 1
	eds.Spec.Scaling.MinReplicas = 4

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 1, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 1, Index: "ad2"},
	}
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(4), *actual.NodeReplicas, actual.Description)
	require.Equal(t, 0, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, NONE, actual.ScalingDirection, actual.Description)
}

func TestAtMaxReplicas(t *testing.T) {
	eds := edsTestFixture(4)
	esNodes := make([]ESNode, 0)

	// don't scale down if we reached MinReplicas
	eds.Spec.Scaling.MaxShardsPerNode = 2
	eds.Spec.Scaling.MinIndexReplicas = 1
	eds.Spec.Scaling.MaxReplicas = 4

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 3, Index: "ad1"},
		"ad2": {Replicas: 1, Primaries: 3, Index: "ad2"},
	}
	scalingHint := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Equal(t, int32(4), *actual.NodeReplicas, actual.Description)
	require.Equal(t, 0, len(actual.IndexReplicas), actual.Description)
	require.Equal(t, NONE, actual.ScalingDirection, actual.Description)
}

func TestAtMaxDisk(t *testing.T) {
	eds := edsTestFixture(4)
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
	eds.Spec.Scaling.MaxShardsPerNode = 12
	eds.Spec.Scaling.MinIndexReplicas = 1
	eds.Spec.Scaling.MaxReplicas = 2
	eds.Spec.Scaling.MinReplicas = 1

	esIndices := map[string]ESIndex{
		"ad1": {Replicas: 1, Primaries: 6, Index: "ad1"},
	}
	scalingHint := DOWN

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(esIndices, esNodes, scalingHint)
	require.Nil(t, actual.NodeReplicas, actual.Description)
	require.Equal(t, actual.ScalingDirection, NONE, actual.Description)
}

func TestEDSWithoutReplicas(t *testing.T) {
	eds := &zv1.ElasticsearchDataSet{}
	scalingHint := UP

	as := systemUnderTest(eds, nil, nil)

	actual := as.calculateScalingOperation(map[string]ESIndex{}, make([]ESNode, 0), scalingHint)
	require.Nil(t, actual.NodeReplicas, actual.Description)
	require.Equal(t, actual.ScalingDirection, NONE, actual.Description)
}

func edsTestFixture(initialReplicas int) *zv1.ElasticsearchDataSet {
	r := int32(initialReplicas)
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
				DiskUsagePercentScaledownWatermark: 75,
			},
			Replicas: &r,
		},
	}
}

func TestCalculateNodeBoundaries(t *testing.T) {
	eds := edsTestFixture(3)
	eds.Spec.Scaling.MinReplicas = 2
	eds.Spec.Scaling.MaxReplicas = 5
	as := systemUnderTest(eds, nil, nil)
	require.Equal(t, 2, int(as.ensureBoundsNodeReplicas(1)))
	require.Equal(t, 3, int(as.ensureBoundsNodeReplicas(3)))
	require.Equal(t, 5, int(as.ensureBoundsNodeReplicas(6)))
}

func TestCalculateIncreasedNodes(t *testing.T) {
	require.Equal(t, 64, int(calculateIncreasedNodes(32, 64)))
	require.Equal(t, 64, int(calculateIncreasedNodes(64, 64)))
	require.Equal(t, 32, int(calculateIncreasedNodes(31, 64)))
}

func TestCalculateDecreaseNodes(t *testing.T) {
	require.Equal(t, 16, int(calculateDecreasedNodes(32, 32)))
	require.Equal(t, 16, int(calculateDecreasedNodes(17, 32)))
	require.Equal(t, 1, int(calculateDecreasedNodes(1, 32)))
}

func TestCalculateNodesWithSameShardToNodeRatio(t *testing.T) {
	require.Equal(t, 16, int(calculateNodesWithSameShardToNodeRatio(16, 4, 4)))
	require.Equal(t, 16, int(calculateNodesWithSameShardToNodeRatio(16, 4, 6)))
	require.Equal(t, 12, int(calculateNodesWithSameShardToNodeRatio(16, 32, 24)))
	require.Equal(t, 17, int(calculateNodesWithSameShardToNodeRatio(17, 32, 32)))
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

func TestGetManagedNodes(t *testing.T) {
	pods := []v1.Pod{
		{
			Status: v1.PodStatus{
				PodIP: "1.2.3.4",
			},
		},
		{
			Status: v1.PodStatus{},
		},
	}
	nodes := []ESNode{
		{
			IP: "1.2.3.4",
		},
		{
			IP: "1.2.3.5",
		},
	}

	as := systemUnderTest(edsTestFixture(1), nil, pods)
	actual := as.getManagedNodes(pods, nodes)

	require.Equal(t, 1, len(actual))
	require.Equal(t, "1.2.3.4", actual[0].IP)
}

func systemUnderTest(eds *zv1.ElasticsearchDataSet, metricSet *zv1.ElasticsearchMetricSet, pods []v1.Pod) *AutoScaler {
	es := &ESResource{
		ElasticsearchDataSet: eds,
		MetricSet:            metricSet,
		Pods:                 pods,
	}
	return NewAutoScaler(es, time.Second*60, nil)
}
