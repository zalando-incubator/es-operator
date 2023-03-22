package main

import (
	"strings"
	"testing"

	"github.com/cenk/backoff"
	"github.com/stretchr/testify/require"
	zv1 "github.com/zalando-incubator/es-operator/pkg/apis/zalando.org/v1"
)

func TestEDSCPUAutoscaleUP8(t *testing.T) {
	t.Parallel()
	runTestEDSCPUAutoScaleUP(t, "8.6.2", "es8-config")
}

func runTestEDSCPUAutoScaleUP(t *testing.T, version, configMap string) {
	edsName := "cpu-autoscale-up-" + strings.Replace(version, ".", "", -1)
	edsSpecFactory := NewTestEDSSpecFactory(edsName, version, configMap)
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
	edsSpec.Template.Spec = edsPodSpecCPULoadContainer(edsName, version, configMap)

	err := createEDS(edsName, edsSpec)
	require.NoError(t, err)

	esClient, err := setupESClient("http://"+edsName+":9200", version)
	require.NoError(t, err)
	createIndex := func() error {
		return esClient.CreateIndex(edsName, edsName, 1, 0)
	}
	backoffCfg := backoff.NewExponentialBackOff()
	err = backoff.Retry(createIndex, backoffCfg)
	require.NoError(t, err)
	verifyEDS(t, edsName, edsSpec, pint32(3))
	err = esClient.DeleteIndex(edsName)
	require.NoError(t, err)
	err = deleteEDS(edsName)
	require.NoError(t, err)
}

func TestEDSAutoscaleUPOnShardCount6(t *testing.T) {
	t.Parallel()
	runTestEDSAutoscaleUPOnShardCount(t, "8.6.2", "es8-config")
}

func TestEDSAutoscaleUPOnShardCount7(t *testing.T) {
	t.Parallel()
	runTestEDSAutoscaleUPOnShardCount(t, "7.17.2", "es7-config")
}

func runTestEDSAutoscaleUPOnShardCount(t *testing.T, version, configMap string) {
	edsName := "shard-autoscale-up-" + strings.Replace(version, ".", "", -1)
	edsSpecFactory := NewTestEDSSpecFactory(edsName, version, configMap)
	edsSpecFactory.Scaling(&zv1.ElasticsearchDataSetScaling{
		Enabled:                            true,
		MinReplicas:                        1,
		MaxReplicas:                        2,
		MinIndexReplicas:                   0,
		MaxIndexReplicas:                   1,
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

	esClient, err := setupESClient("http://"+edsName+":9200", version)
	require.NoError(t, err)
	createIndex := func() error {
		return esClient.CreateIndex(edsName, edsName, 2, 0)
	}
	backoffCfg := backoff.NewExponentialBackOff()
	err = backoff.Retry(createIndex, backoffCfg)
	require.NoError(t, err)
	verifyEDS(t, edsName, edsSpec, pint32(2))
	err = esClient.DeleteIndex(edsName)
	require.NoError(t, err)
	err = deleteEDS(edsName)
	require.NoError(t, err)
}
