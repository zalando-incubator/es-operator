package operator

import (
	"fmt"
	"math"
	"time"

	log "github.com/sirupsen/logrus"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	"k8s.io/api/core/v1"
)

// 1. check if we have enough data
// 2. decide if we need to scale up or down
// 3. retrieve elasticsearch data (indices, shards, replicas)
// 4. decide if we only need to increase/decrease nodes
//    -> if not, don't do anything for now...

type ScalingDirection int

func (d ScalingDirection) String() string {
	switch d {
	case DOWN:
		return "DOWN"
	case UP:
		return "UP"
	case NONE:
		return "NONE"
	}
	return ""
}

const (
	DOWN ScalingDirection = iota
	NONE
	UP
)

type ScalingOperation struct {
	ScalingDirection ScalingDirection
	NodeReplicas     *int32
	IndexReplicas    []ESIndex
	Description      string
}

func noopScalingOperation(description string) *ScalingOperation {
	return &ScalingOperation{
		ScalingDirection: NONE,
		Description:      description,
	}
}

func getScalingDirection(eds *zv1.ElasticsearchDataSet, esMSet *zv1.ElasticsearchMetricSet, metricsInterval time.Duration) ScalingDirection {
	scaling := eds.Spec.Scaling
	name := eds.Name
	namespace := eds.Namespace

	// no metrics yet
	if esMSet == nil {
		return NONE
	}

	status := eds.Status

	// TODO: only consider metric samples that are not too old.
	sampleSize := len(esMSet.Metrics)

	// check for enough data points
	requiredScaledownSamples := int(math.Ceil(float64(scaling.ScaleDownThresholdDurationSeconds) / metricsInterval.Seconds()))
	if sampleSize >= requiredScaledownSamples {
		// check if CPU is below threshold for the last n samples
		scaleDownRequired := true
		for _, currentItem := range esMSet.Metrics[sampleSize-requiredScaledownSamples:] {
			if currentItem.Value >= scaling.ScaleDownCPUBoundary {
				scaleDownRequired = false
				break
			}
		}
		if scaleDownRequired {
			if status.LastScaleDownStarted == nil || status.LastScaleDownStarted.Time.Before(time.Now().Add(-time.Duration(scaling.ScaleDownCooldownSeconds)*time.Second)) {
				log.Infof("EDS %s/%s scaling hint: %s", namespace, name, DOWN)
				return DOWN
			}
			log.Infof("EDS %s/%s not scaling down, currently in cool-down period.", namespace, name)
		}
	}

	requiredScaleupSamples := int(math.Ceil(float64(scaling.ScaleUpThresholdDurationSeconds) / metricsInterval.Seconds()))
	if sampleSize >= requiredScaleupSamples {
		// check if CPU is above threshold for the last n samples
		scaleUpRequired := true
		for _, currentItem := range esMSet.Metrics[sampleSize-requiredScaleupSamples:] {
			if currentItem.Value <= scaling.ScaleUpCPUBoundary {
				scaleUpRequired = false
				break
			}
		}
		if scaleUpRequired {
			if status.LastScaleUpStarted == nil || status.LastScaleUpStarted.Time.Before(time.Now().Add(-time.Duration(scaling.ScaleUpCooldownSeconds)*time.Second)) {
				log.Infof("EDS %s/%s scaling hint: %s", namespace, name, UP)
				return UP
			}
			log.Infof("EDS %s/%s not scaling up, currently in cool-down period.", namespace, name)
		}
	}
	return NONE
}

// TODO: check alternative approach by configuring the tags used for `index.routing.allocation`
// and deriving the indices from there.
func getScalingOperation(eds *zv1.ElasticsearchDataSet, pods []v1.Pod, direction ScalingDirection, client *ESClient) (*ScalingOperation, error) {
	esIndices, err := client.GetIndices()
	if err != nil {
		return nil, err
	}

	esShards, err := client.GetShards()
	if err != nil {
		return nil, err
	}

	esNodes, err := client.GetNodes()
	if err != nil {
		return nil, err
	}

	managedIndices := getManagedIndices(pods, esIndices, esShards)
	managedNodes := getManagedNodes(pods, esNodes)
	return calculateScalingOperation(eds, managedIndices, managedNodes, direction), nil
}

func getManagedNodes(pods []v1.Pod, esNodes []ESNode) []ESNode {
	podIPs := make(map[string]struct{})
	for _, pod := range pods {
		if pod.Status.PodIP != "" {
			podIPs[pod.Status.PodIP] = struct{}{}
		}
	}
	managedNodes := make([]ESNode, 0, len(pods))
	for _, node := range esNodes {
		if _, ok := podIPs[node.IP]; ok {
			managedNodes = append(managedNodes, node)
		}
	}
	return managedNodes
}

func getManagedIndices(pods []v1.Pod, esIndices []ESIndex, esShards []ESShard) map[string]ESIndex {
	podIPs := make(map[string]struct{})
	for _, pod := range pods {
		if pod.Status.PodIP != "" {
			podIPs[pod.Status.PodIP] = struct{}{}
		}
	}
	managedIndices := make(map[string]ESIndex)
	for _, shard := range esShards {
		if _, ok := podIPs[shard.IP]; ok {
			for _, index := range esIndices {
				if shard.Index == index.Index {
					managedIndices[shard.Index] = index
					break
				}
			}
		}
	}
	return managedIndices
}

func calculateScalingOperation(eds *zv1.ElasticsearchDataSet, managedIndices map[string]ESIndex, managedNodes []ESNode, direction ScalingDirection) *ScalingOperation {
	scalingSpec := eds.Spec.Scaling

	currentDesiredReplicas := eds.Spec.Replicas
	if currentDesiredReplicas == nil {
		return noopScalingOperation("DesiredReplicas is not set yet.")
	}

	if len(managedIndices) == 0 {
		return noopScalingOperation("No indices allocated yet.")
	}

	scalingOperation := scaleUpOrDown(eds, managedIndices, direction, *currentDesiredReplicas)

	// safety check: ensure we don't scale below minIndexReplicas+1
	if scalingOperation.NodeReplicas != nil && *scalingOperation.NodeReplicas < scalingSpec.MinIndexReplicas+1 {
		return noopScalingOperation(fmt.Sprintf("Scaling would violate the minimum required nodes to hold %d index replicas.", scalingSpec.MinIndexReplicas))
	}

	// safety check: ensure we don't scale-down if disk usage is already above threshold
	if scalingOperation.ScalingDirection == DOWN && scalingSpec.DiskUsagePercentScaledownWatermark > 0 && getMaxDiskUsage(managedNodes) > scalingSpec.DiskUsagePercentScaledownWatermark {
		return noopScalingOperation(fmt.Sprintf("Scaling would violate the minimum required disk free percent: %.2f", 75.0))
	}

	return scalingOperation
}

func getMaxDiskUsage(managedNodes []ESNode) float64 {
	maxDisk := 0.0
	for _, node := range managedNodes {
		maxDisk = math.Max(maxDisk, node.DiskUsedPercent)
	}
	return maxDisk
}

func ensureLowerBoundNodeReplicas(scalingSpec *zv1.ElasticsearchDataSetScaling, newDesiredNodeReplicas int32) int32 {
	if scalingSpec.MinReplicas > 0 {
		return int32(math.Max(float64(newDesiredNodeReplicas), float64(scalingSpec.MinReplicas)))
	}
	return newDesiredNodeReplicas
}

func ensureUpperBoundNodeReplicas(scalingSpec *zv1.ElasticsearchDataSetScaling, newDesiredNodeReplicas int32) int32 {
	if scalingSpec.MaxReplicas > 0 {
		return int32(math.Min(float64(newDesiredNodeReplicas), float64(scalingSpec.MaxReplicas)))
	}
	return newDesiredNodeReplicas
}

func scaleUpOrDown(eds *zv1.ElasticsearchDataSet, esIndices map[string]ESIndex, direction ScalingDirection, currentDesiredReplicas int32) *ScalingOperation {
	scalingSpec := eds.Spec.Scaling
	name := eds.Name
	namespace := eds.Namespace

	currentTotalShards := int32(0)
	for _, index := range esIndices {
		log.Debugf("EDS %s/%s - index: %s, primaries: %d, replicas: %d", namespace, name, index.Index, index.Primaries, index.Replicas)
		currentTotalShards += index.Primaries * (index.Replicas + 1)
	}

	currentShardToNodeRatio := float64(currentTotalShards) / float64(currentDesiredReplicas)

	// independent of the scaling direction: in case the scaling settings have changed (e.g. the MaxShardsPerNode), we might need to scale up.
	if currentShardToNodeRatio > float64(scalingSpec.MaxShardsPerNode) {
		newDesiredNodeReplicas := ensureUpperBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(currentTotalShards)/float64(scalingSpec.MaxShardsPerNode))))
		return &ScalingOperation{
			ScalingDirection: UP,
			NodeReplicas:     &newDesiredNodeReplicas,
			Description:      fmt.Sprintf("Current shard-to-node ratio (%.2f) exceeding the desired limit of (%d).", currentShardToNodeRatio, scalingSpec.MaxShardsPerNode),
		}
	}

	newDesiredIndexReplicas := make([]ESIndex, 0, len(esIndices))

	switch direction {
	case UP:
		if currentShardToNodeRatio <= float64(scalingSpec.MinShardsPerNode) {
			newTotalShards := currentTotalShards
			for _, index := range esIndices {
				if index.Replicas >= scalingSpec.MaxIndexReplicas {
					return noopScalingOperation(fmt.Sprintf("Not allowed to scale up due to maxIndexReplicas (%d) reached for index %s.",
						scalingSpec.MaxIndexReplicas, index.Index))
				} else {
					newTotalShards += index.Primaries
					newDesiredIndexReplicas = append(newDesiredIndexReplicas, ESIndex{
						Index:     index.Index,
						Primaries: index.Primaries,
						Replicas:  index.Replicas + 1,
					})
				}
			}
			if newTotalShards != currentTotalShards {
				newDesiredNodeReplicas := ensureUpperBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(newTotalShards)/float64(currentShardToNodeRatio))))
				return &ScalingOperation{
					Description:      fmt.Sprintf("Keeping shard-to-node ratio (%.2f), and increasing index replicas.", currentShardToNodeRatio),
					NodeReplicas:     &newDesiredNodeReplicas,
					IndexReplicas:    newDesiredIndexReplicas,
					ScalingDirection: direction,
				}
			}
		}

		newDesiredNodeReplicas := ensureUpperBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(currentTotalShards)/float64(currentShardToNodeRatio-1))))

		return &ScalingOperation{
			ScalingDirection: direction,
			NodeReplicas:     &newDesiredNodeReplicas,
			Description:      fmt.Sprintf("Increasing node replicas to %d.", newDesiredNodeReplicas),
		}
	case DOWN:
		newTotalShards := currentTotalShards
		for _, index := range esIndices {
			if index.Replicas > scalingSpec.MinIndexReplicas {
				newTotalShards -= index.Primaries
				newDesiredIndexReplicas = append(newDesiredIndexReplicas, ESIndex{
					Index:     index.Index,
					Primaries: index.Primaries,
					Replicas:  index.Replicas - 1,
				})
			}
		}
		if newTotalShards != currentTotalShards {
			newDesiredNodeReplicas := ensureLowerBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(newTotalShards)/float64(currentShardToNodeRatio))))
			return &ScalingOperation{
				ScalingDirection: direction,
				NodeReplicas:     &newDesiredNodeReplicas,
				IndexReplicas:    newDesiredIndexReplicas,
				Description:      fmt.Sprintf("Keeping shard-to-node ratio (%.2f), and decreasing index replicas.", currentShardToNodeRatio),
			}
		}
		// increase shard-to-node ratio, and scale down by at least one
		newDesiredNodeReplicas := ensureLowerBoundNodeReplicas(scalingSpec,
			int32(math.Min(float64(currentDesiredReplicas)-float64(1), math.Ceil(float64(currentTotalShards)/float64(currentShardToNodeRatio+1)))))
		ratio := float64(newTotalShards) / float64(newDesiredNodeReplicas)
		if ratio >= float64(scalingSpec.MaxShardsPerNode) {
			return noopScalingOperation(fmt.Sprintf("Scaling would violate the shard-to-node maximum (%.2f/%d).", ratio, scalingSpec.MaxShardsPerNode))
		}

		return &ScalingOperation{
			ScalingDirection: direction,
			NodeReplicas:     &newDesiredNodeReplicas,
			Description:      fmt.Sprintf("Decreasing node replicas to %d.", newDesiredNodeReplicas),
		}
	}
	return noopScalingOperation("Nothing to do")
}
