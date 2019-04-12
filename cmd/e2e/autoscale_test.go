package main

import (
	"testing"

	"github.com/cenk/backoff"
	"github.com/stretchr/testify/require"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
)

func TestEDSCPUAutoscaleUP(t *testing.T) {
	t.Parallel()
	edsName := "cpu-autoscale-up"
	edsSpecFactory := NewTestEDSSpecFactory(edsName)
	edsSpecFactory.Scaling(&zv1.ElasticsearchDataSetScaling{
		Enabled:                            true,
		MinReplicas:                        1,
		MaxReplicas:                        3,
		MinIndexReplicas:                   0,
		MaxIndexReplicas:                   2,
		MinShardsPerNode:                   1,
		MaxShardsPerNode:                   1,
		ScaleUpCPUBoundary:                 50,
		ScaleUpThresholdDurationSeconds:    60,
		ScaleUpCooldownSeconds:             0,
		ScaleDownCPUBoundary:               1,
		ScaleDownThresholdDurationSeconds:  600,
		ScaleDownCooldownSeconds:           600,
		DiskUsagePercentScaledownWatermark: 0,
	})
	edsSpec := edsSpecFactory.Create()
	edsSpec.Template.Spec = edsPodSpecCPULoadContainer(edsName)

	err := createEDS(edsName, edsSpec)
	require.NoError(t, err)

	esClient, err := setupESClient("http://" + edsName + ":9200")
	require.NoError(t, err)
	createIndex := func() error {
		return esClient.CreateIndex(edsName, edsName, 1, 0)
	}
	backoffCfg := backoff.NewExponentialBackOff()
	err = backoff.Retry(createIndex, backoffCfg)
	require.NoError(t, err)
	verifyEDS(t, edsName, edsSpec, pint32(3))
	esClient.DeleteIndex(edsName)
	deleteEDS(edsName)
}

func TestEDSAutoscaleUPOnShardCount(t *testing.T) {
	t.Parallel()
	edsName := "shard-autoscale-up"
	edsSpecFactory := NewTestEDSSpecFactory(edsName)
	edsSpecFactory.Scaling(&zv1.ElasticsearchDataSetScaling{
		Enabled:                            true,
		MinReplicas:                        1,
		MaxReplicas:                        3,
		MinIndexReplicas:                   0,
		MaxIndexReplicas:                   2,
		MinShardsPerNode:                   1,
		MaxShardsPerNode:                   1,
		ScaleUpCPUBoundary:                 50,
		ScaleUpThresholdDurationSeconds:    120,
		ScaleUpCooldownSeconds:             30,
		ScaleDownCPUBoundary:               20,
		ScaleDownThresholdDurationSeconds:  120,
		ScaleDownCooldownSeconds:           30,
		DiskUsagePercentScaledownWatermark: 0,
	})
	edsSpec := edsSpecFactory.Create()

	err := createEDS(edsName, edsSpec)
	require.NoError(t, err)

	esClient, err := setupESClient("http://" + edsName + ":9200")
	require.NoError(t, err)
	createIndex := func() error {
		return esClient.CreateIndex(edsName, edsName, 2, 0)
	}
	backoffCfg := backoff.NewExponentialBackOff()
	err = backoff.Retry(createIndex, backoffCfg)
	require.NoError(t, err)
	verifyEDS(t, edsName, edsSpec, pint32(2))
	esClient.DeleteIndex(edsName)
	deleteEDS(edsName)
}
