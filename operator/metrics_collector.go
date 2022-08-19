package operator

import (
	"context"
	"math"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	v12 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
	"github.com/zalando-incubator/es-operator/pkg/clientset"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type ElasticsearchMetricsCollector struct {
	logger *log.Entry
	kube   *clientset.Clientset
	es     ESResource
}

func (c *ElasticsearchMetricsCollector) collectMetrics(ctx context.Context) error {
	// first, collect metrics for all pods....
	metrics, err := c.kube.MetricsV1Beta1().PodMetricses(c.es.ElasticsearchDataSet.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	cpuUsagePercent := getCPUUsagePercent(metrics.Items, c.es.Pods)

	if len(cpuUsagePercent) == 0 {
		c.logger.Debug("Didn't have any metrics to collect.")
		return nil
	}

	// calculate its median
	median := calculateMedian(cpuUsagePercent)

	// next, get the configmap and store the sample
	return c.storeMedian(ctx, median)
}

func getCPUUsagePercent(metrics []v1beta1.PodMetrics, pods []v1.Pod) []int32 {
	podResources := make(map[string]map[string]v1.ResourceRequirements, len(pods))
	for _, pod := range pods {
		containerMap := make(map[string]v1.ResourceRequirements, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			containerMap[container.Name] = container.Resources
		}
		podResources[pod.Name] = containerMap
	}

	// calculate the max usage/request value for each pod by looking at all
	// containers in every pod.
	cpuUsagePercent := []int32{}
	for _, podMetrics := range metrics {
		if containerResources, ok := podResources[podMetrics.Name]; ok {
			podMax := int32(0)
			for _, container := range podMetrics.Containers {
				if resources, ok := containerResources[container.Name]; ok {
					requestedCPU := resources.Requests.Cpu().MilliValue()
					usageCPU := container.Usage.Cpu().MilliValue()
					if requestedCPU > 0 {
						usagePcnt := int32(100 * usageCPU / requestedCPU)
						if usagePcnt > podMax {
							podMax = usagePcnt
						}
					}
				}
			}
			cpuUsagePercent = append(cpuUsagePercent, podMax)
		}
	}
	return cpuUsagePercent
}

func (c *ElasticsearchMetricsCollector) storeMedian(ctx context.Context, i int32) error {
	currentValue := v12.ElasticsearchMetric{
		Timestamp: metav1.Now(),
		Value:     i,
	}

	if c.es.MetricSet == nil {
		c.es.MetricSet = &v12.ElasticsearchMetricSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.es.ElasticsearchDataSet.Name,
				Namespace: c.es.ElasticsearchDataSet.Namespace,
				Labels:    c.es.ElasticsearchDataSet.Labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: c.es.ElasticsearchDataSet.APIVersion,
						Kind:       c.es.ElasticsearchDataSet.Kind,
						Name:       c.es.ElasticsearchDataSet.Name,
						UID:        c.es.ElasticsearchDataSet.UID,
					},
				},
			},
			Metrics: []v12.ElasticsearchMetric{currentValue},
		}
		_, err := c.kube.ZalandoV1().ElasticsearchMetricSets(c.es.MetricSet.Namespace).Create(ctx, c.es.MetricSet, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		threshold := time.Duration(math.Max(float64(c.es.ElasticsearchDataSet.Spec.Scaling.ScaleDownThresholdDurationSeconds), float64(c.es.ElasticsearchDataSet.Spec.Scaling.ScaleUpThresholdDurationSeconds))) * time.Second
		oldestTimestamp := time.Now().Add(-threshold)

		newMetricsList := make([]v12.ElasticsearchMetric, 0, len(c.es.MetricSet.Metrics)+1)
		for _, m := range c.es.MetricSet.Metrics {
			if m.Timestamp.Time.Before(oldestTimestamp) {
				continue
			}
			newMetricsList = append(newMetricsList, m)
		}
		newMetricsList = append(newMetricsList, currentValue)
		c.es.MetricSet.Metrics = newMetricsList

		_, err := c.kube.ZalandoV1().ElasticsearchMetricSets(c.es.MetricSet.Namespace).Update(ctx, c.es.MetricSet, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func calculateMedian(cpuMetrics []int32) int32 {
	sort.Slice(cpuMetrics, func(i, j int) bool { return cpuMetrics[i] < cpuMetrics[j] })
	// in case of even number of samples we now get the lower one.
	return cpuMetrics[(len(cpuMetrics)-1)/2]
}
