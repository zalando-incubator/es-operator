package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEDSUpgradingEDS(t *testing.T) {
	t.Parallel()
	edsName := "upgrade"
	edsSpec := testEDSCreate(t, edsName, "6.8.14", "es6-config")
	eds := verifyEDS(t, edsName, edsSpec, edsSpec.Replicas, edsSpec.HpaReplicas)
	// this could become a test for a major version upgrade in the future.
	eds.Spec.Template.Spec.Containers[0].Image = "docker.elastic.co/elasticsearch/elasticsearch-oss:6.7.2"

	var err error
	eds, err = waitForEDS(t, edsName)
	require.NoError(t, err)
	err = updateEDS(edsName, eds)
	require.NoError(t, err)

	verifyEDS(t, edsName, eds.Spec, eds.Spec.Replicas, edsSpec.HpaReplicas)
	err = deleteEDS(edsName)
	require.NoError(t, err)
}
