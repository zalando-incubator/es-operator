package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEDSUpgradingEDS(t *testing.T) {
	t.Parallel()
	edsName := "upgrade"
	edsSpec := testEDSCreate(t, edsName)
	eds := verifyEDS(t, edsName, edsSpec, edsSpec.Replicas)
	eds.Spec.Template.Labels["new-label"] = "hello"

	err := updateEDS(edsName, eds)
	require.NoError(t, err)

	verifyEDS(t, edsName, eds.Spec, eds.Spec.Replicas)
	deleteEDS(edsName)
}
