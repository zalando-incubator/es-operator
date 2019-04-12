package operator

import (
	"fmt"
	"math"
	"time"

	log "github.com/sirupsen/logrus"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	v1 "k8s.io/api/core/v1"
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

type AutoScaler struct {
	logger          *log.Entry
	eds             *zv1.ElasticsearchDataSet
	esMSet          *zv1.ElasticsearchMetricSet
	metricsInterval time.Duration
	pods            []v1.Pod
	esClient        *ESClient
}

func NewAutoScaler(es *ESResource, metricsInterval time.Duration, esClient *ESClient) *AutoScaler {
	return &AutoScaler{
		logger: log.WithFields(log.Fields{
			"eds":       es.ElasticsearchDataSet.Name,
			"namespace": es.ElasticsearchDataSet.Namespace,
		}),
		eds:             es.ElasticsearchDataSet,
		esMSet:          es.MetricSet,
		metricsInterval: metricsInterval,
		pods:            es.Pods,
		esClient:        esClient,
	}
}

func (as *AutoScaler) getScalingDirection() ScalingDirection {
	scaling := as.eds.Spec.Scaling

	// no metrics yet
	if as.esMSet == nil {
		return NONE
	}

	status := as.eds.Status

	// TODO: only consider metric samples that are not too old.
	sampleSize := len(as.esMSet.Metrics)

	// check for enough data points
	requiredScaledownSamples := int(math.Ceil(float64(scaling.ScaleDownThresholdDurationSeconds) / as.metricsInterval.Seconds()))
	if sampleSize >= requiredScaledownSamples {
		// check if CPU is below threshold for the last n samples
		scaleDownRequired := true
		for _, currentItem := range as.esMSet.Metrics[sampleSize-requiredScaledownSamples:] {
			if currentItem.Value >= scaling.ScaleDownCPUBoundary {
				scaleDownRequired = false
				break
			}
		}
		if scaleDownRequired {
			if status.LastScaleDownStarted == nil || status.LastScaleDownStarted.Time.Before(time.Now().Add(-time.Duration(scaling.ScaleDownCooldownSeconds)*time.Second)) {
				as.logger.Infof("Scaling hint: %s", DOWN)
				return DOWN
			}
			as.logger.Info("Not scaling down, currently in cool-down period.")
		}
	}

	requiredScaleupSamples := int(math.Ceil(float64(scaling.ScaleUpThresholdDurationSeconds) / as.metricsInterval.Seconds()))
	if sampleSize >= requiredScaleupSamples {
		// check if CPU is above threshold for the last n samples
		scaleUpRequired := true
		for _, currentItem := range as.esMSet.Metrics[sampleSize-requiredScaleupSamples:] {
			if currentItem.Value <= scaling.ScaleUpCPUBoundary {
				scaleUpRequired = false
				break
			}
		}
		if scaleUpRequired {
			if status.LastScaleUpStarted == nil || status.LastScaleUpStarted.Time.Before(time.Now().Add(-time.Duration(scaling.ScaleUpCooldownSeconds)*time.Second)) {
				as.logger.Infof("Scaling hint: %s", UP)
				return UP
			}
			as.logger.Info("Not scaling up, currently in cool-down period.")
		}
	}
	return NONE
}

// TODO: check alternative approach by configuring the tags used for `index.routing.allocation`
// and deriving the indices from there.
func (as *AutoScaler) GetScalingOperation() (*ScalingOperation, error) {
	direction := as.getScalingDirection()
	esIndices, err := as.esClient.GetIndices()
	if err != nil {
		return nil, err
	}

	esShards, err := as.esClient.GetShards()
	if err != nil {
		return nil, err
	}

	esNodes, err := as.esClient.GetNodes()
	if err != nil {
		return nil, err
	}

	managedIndices := as.getManagedIndices(esIndices, esShards)
	managedNodes := as.getManagedNodes(as.pods, esNodes)
	return as.calculateScalingOperation(managedIndices, managedNodes, direction), nil
}

func (as *AutoScaler) getManagedNodes(pods []v1.Pod, esNodes []ESNode) []ESNode {
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

func (as *AutoScaler) getManagedIndices(esIndices []ESIndex, esShards []ESShard) map[string]ESIndex {
	podIPs := make(map[string]struct{})
	for _, pod := range as.pods {
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

func (as *AutoScaler) calculateScalingOperation(managedIndices map[string]ESIndex, managedNodes []ESNode, direction ScalingDirection) *ScalingOperation {
	scalingSpec := as.eds.Spec.Scaling

	currentDesiredReplicas := as.eds.Spec.Replicas
	if currentDesiredReplicas == nil {
		return noopScalingOperation("DesiredReplicas is not set yet.")
	}

	if len(managedIndices) == 0 {
		return noopScalingOperation("No indices allocated yet.")
	}

	scalingOperation := as.scaleUpOrDown(managedIndices, direction, *currentDesiredReplicas)

	// safety check: ensure we don't scale below minIndexReplicas+1
	if scalingOperation.NodeReplicas != nil && *scalingOperation.NodeReplicas < scalingSpec.MinIndexReplicas+1 {
		return noopScalingOperation(fmt.Sprintf("Scaling would violate the minimum required nodes to hold %d index replicas.", scalingSpec.MinIndexReplicas))
	}

	// safety check: ensure we don't scale-down if disk usage is already above threshold
	if scalingOperation.ScalingDirection == DOWN && scalingSpec.DiskUsagePercentScaledownWatermark > 0 && as.getMaxDiskUsage(managedNodes) > scalingSpec.DiskUsagePercentScaledownWatermark {
		return noopScalingOperation(fmt.Sprintf("Scaling would violate the minimum required disk free percent: %.2f", 75.0))
	}

	return scalingOperation
}

func (as *AutoScaler) getMaxDiskUsage(managedNodes []ESNode) float64 {
	maxDisk := 0.0
	for _, node := range managedNodes {
		maxDisk = math.Max(maxDisk, node.DiskUsedPercent)
	}
	return maxDisk
}

func (as *AutoScaler) ensureLowerBoundNodeReplicas(scalingSpec *zv1.ElasticsearchDataSetScaling, newDesiredNodeReplicas int32) int32 {
	if scalingSpec.MinReplicas > 0 {
		return int32(math.Max(float64(newDesiredNodeReplicas), float64(scalingSpec.MinReplicas)))
	}
	return newDesiredNodeReplicas
}

func (as *AutoScaler) ensureUpperBoundNodeReplicas(scalingSpec *zv1.ElasticsearchDataSetScaling, newDesiredNodeReplicas int32) int32 {
	if scalingSpec.MaxReplicas > 0 {
		upperBound := int32(math.Min(float64(newDesiredNodeReplicas), float64(scalingSpec.MaxReplicas)))
		if upperBound < newDesiredNodeReplicas {
			as.logger.Warnf("Requested to scale up to %d, which is beyond the defined maxReplicas of %d.", newDesiredNodeReplicas, scalingSpec.MaxReplicas)
		}
		return upperBound
	}
	return newDesiredNodeReplicas
}

func (as *AutoScaler) scaleUpOrDown(esIndices map[string]ESIndex, direction ScalingDirection, currentDesiredReplicas int32) *ScalingOperation {
	scalingSpec := as.eds.Spec.Scaling

	currentTotalShards := int32(0)
	for _, index := range esIndices {
		as.logger.Debugf("Index: %s, primaries: %d, replicas: %d", index.Index, index.Primaries, index.Replicas)
		currentTotalShards += index.Primaries * (index.Replicas + 1)
	}

	currentShardToNodeRatio := float64(currentTotalShards) / float64(currentDesiredReplicas)

	// independent of the scaling direction: in case the scaling settings have changed (e.g. the MaxShardsPerNode), we might need to scale up.
	if currentShardToNodeRatio > float64(scalingSpec.MaxShardsPerNode) {
		newDesiredNodeReplicas := as.ensureUpperBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(currentTotalShards)/float64(scalingSpec.MaxShardsPerNode))))
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
				}
				newTotalShards += index.Primaries
				newDesiredIndexReplicas = append(newDesiredIndexReplicas, ESIndex{
					Index:     index.Index,
					Primaries: index.Primaries,
					Replicas:  index.Replicas + 1,
				})
			}
			if newTotalShards != currentTotalShards {
				newDesiredNodeReplicas := as.ensureUpperBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(newTotalShards)/float64(currentShardToNodeRatio))))
				return &ScalingOperation{
					Description:      fmt.Sprintf("Keeping shard-to-node ratio (%.2f), and increasing index replicas.", currentShardToNodeRatio),
					NodeReplicas:     &newDesiredNodeReplicas,
					IndexReplicas:    newDesiredIndexReplicas,
					ScalingDirection: direction,
				}
			}
		}

		newDesiredNodeReplicas := as.ensureUpperBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(currentTotalShards)/float64(currentShardToNodeRatio-1))))

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
			newDesiredNodeReplicas := as.ensureLowerBoundNodeReplicas(scalingSpec, int32(math.Ceil(float64(newTotalShards)/float64(currentShardToNodeRatio))))
			return &ScalingOperation{
				ScalingDirection: direction,
				NodeReplicas:     &newDesiredNodeReplicas,
				IndexReplicas:    newDesiredIndexReplicas,
				Description:      fmt.Sprintf("Keeping shard-to-node ratio (%.2f), and decreasing index replicas.", currentShardToNodeRatio),
			}
		}
		// increase shard-to-node ratio, and scale down by at least one
		newDesiredNodeReplicas := as.ensureLowerBoundNodeReplicas(scalingSpec,
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
