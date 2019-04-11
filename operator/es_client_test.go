package operator

import (
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestDrain(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/settings",
		httpmock.NewStringResponder(200, `{"transient":{"cluster":{"routing":{"rebalance":{"enable":"all"}}}}}`))
	httpmock.RegisterResponder("PUT", "http://elasticsearch:9200/_cluster/settings",
		httpmock.NewStringResponder(200, `{}`))
	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/health",
		httpmock.NewStringResponder(200, `{"status":"green"}`))
	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/shards",
		httpmock.NewStringResponder(200, `[{"index":"a","ip":"10.2.19.5"},{"index":"b","ip":"10.2.10.2"},{"index":"c","ip":"10.2.16.2"}]`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	err := systemUnderTest.Drain(nil, &v1.Pod{
		Status: v1.PodStatus{
			PodIP: "1.2.3.4",
		},
	})

	assert.NoError(t, err)

	info := httpmock.GetCallCountInfo()
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cluster/health"])
	require.EqualValues(t, 2, info["PUT http://elasticsearch:9200/_cluster/settings"])
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cluster/settings"])
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cat/shards"])
}

func TestCleanup(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/nodes",
		httpmock.NewStringResponder(200, `[{"ip":"10.2.10.2","dup":"22.92"},{"ip":"10.2.16.2","dup":"11.17"},{"ip":"10.2.23.2","dup":"10.97"},{"ip":"10.2.11.3","dup":"18.95"},{"ip":"10.2.25.4","dup":"21.26"},{"ip":"10.2.4.21","dup":"33.19"},{"ip":"10.2.60.19","dup":"21.60"},{"ip":"10.2.19.5","dup":"16.55"},{"ip":"10.2.27.11","dup":"29.80"},{"ip":"10.2.24.13","dup":"31.25"},{"ip":"10.2.18.2","dup":"12.94"}]`))
	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/settings",
		httpmock.NewStringResponder(200, `{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_ip":"2.3.4.5"}}}}}}`))
	httpmock.RegisterResponder("PUT", "http://elasticsearch:9200/_cluster/settings",
		httpmock.NewStringResponder(200, `{}`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	err := systemUnderTest.Cleanup(nil)

	assert.NoError(t, err)

	info := httpmock.GetCallCountInfo()
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cat/nodes"])
	require.EqualValues(t, 2, info["PUT http://elasticsearch:9200/_cluster/settings"])
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cluster/settings"])
}

func TestGetNodes(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/nodes",
		httpmock.NewStringResponder(200, `[{"ip":"10.2.10.2","dup":"22.92"},{"ip":"10.2.16.2","dup":"11.17"},{"ip":"10.2.23.2","dup":"10.97"},{"ip":"10.2.11.3","dup":"18.95"},{"ip":"10.2.25.4","dup":"21.26"},{"ip":"10.2.4.21","dup":"33.19"},{"ip":"10.2.60.19","dup":"21.60"},{"ip":"10.2.19.5","dup":"16.55"},{"ip":"10.2.27.11","dup":"29.80"},{"ip":"10.2.24.13","dup":"31.25"},{"ip":"10.2.18.2","dup":"12.94"}]`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	nodes, err := systemUnderTest.GetNodes()

	assert.NoError(t, err)

	require.EqualValues(t, 11, len(nodes))
	require.EqualValues(t, "10.2.10.2", nodes[0].IP)
	require.EqualValues(t, 22.92, nodes[0].DiskUsedPercent)

}

func TestGetShards(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/shards",
		httpmock.NewStringResponder(200, `[{"index":"a","ip":"10.2.19.5"},{"index":"b","ip":"10.2.10.2"},{"index":"c","ip":"10.2.16.2"}]`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	shards, err := systemUnderTest.GetShards()

	assert.NoError(t, err)

	require.EqualValues(t, 3, len(shards))
	require.EqualValues(t, "10.2.19.5", shards[0].IP)
	require.EqualValues(t, "a", shards[0].Index)

}

func TestGetIndices(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/indices",
		httpmock.NewStringResponder(200, `[{"index":"a","pri":"2","rep":"1"},{"index":"b","pri":"3","rep":"1"},{"index":"c","pri":"6","rep":"1"}]`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	indices, err := systemUnderTest.GetIndices()

	assert.NoError(t, err)

	require.EqualValues(t, 3, len(indices))
	require.EqualValues(t, "a", indices[0].Index)
	require.EqualValues(t, 2, indices[0].Primaries)
	require.EqualValues(t, 1, indices[0].Replicas)

}

func TestUpdateIndexSettings(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/health",
		httpmock.NewStringResponder(200, `{"status":"green"}`))
	httpmock.RegisterResponder("PUT", "http://elasticsearch:9200/myindex/_settings",
		httpmock.NewStringResponder(200, `{}`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	indices := make([]ESIndex, 0, 1)
	indices = append(indices, ESIndex{
		Primaries: 1,
		Replicas:  1,
		Index:     "myindex",
	})
	err := systemUnderTest.UpdateIndexSettings(indices)

	assert.NoError(t, err)

}

func TestCreateIndex(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("PUT", "http://elasticsearch:9200/myindex",
		httpmock.NewStringResponder(200, `{}`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	err := systemUnderTest.CreateIndex("myindex", "mygroup", 2, 2)

	assert.NoError(t, err)

}

func TestDeleteIndex(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("DELETE", "http://elasticsearch:9200/myindex",
		httpmock.NewStringResponder(200, `{}`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	err := systemUnderTest.DeleteIndex("myindex")

	assert.NoError(t, err)
}

func TestEnsureGreenClusterState(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/health",
		httpmock.NewStringResponder(200, `{"status":"yellow"}`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: url,
	}

	err := systemUnderTest.ensureGreenClusterState()

	assert.Error(t, err)
}
