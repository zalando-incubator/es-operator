package operator

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando-incubator/es-operator/operator/null"
	v1 "k8s.io/api/core/v1"
)

func TestDrain(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/settings",
		httpmock.NewStringResponder(200, `{"persistent":{"cluster":{"routing":{"rebalance":{"enable":"all"}}}}}`))
	httpmock.RegisterResponder("PUT", "http://elasticsearch:9200/_cluster/settings",
		httpmock.NewStringResponder(200, `{}`))
	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/health",
		httpmock.NewStringResponder(200, `{"status":"green"}`))
	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/shards",
		httpmock.NewStringResponder(200, `[{"index":"a","ip":"10.2.19.5"},{"index":"b","ip":"10.2.10.2"},{"index":"c","ip":"10.2.16.2"}]`))

	esUrl, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: esUrl,
	}
	err := systemUnderTest.Drain(context.TODO(), &v1.Pod{
		Status: v1.PodStatus{
			PodIP: "1.2.3.4",
		},
	})

	assert.NoError(t, err)

	info := httpmock.GetCallCountInfo()
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cluster/health"])
	require.EqualValues(t, 2, info["PUT http://elasticsearch:9200/_cluster/settings"])
	require.EqualValues(t, 2, info["GET http://elasticsearch:9200/_cluster/settings"])
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cat/shards"])
}

func TestDrainWithTransientSettings(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	var intermediateClusterSettings []byte
	expectedSequenceOfExcludeIPs := []string{"", "1.2.3.4"}
	var numCalls = 0

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/settings",
		func(request *http.Request) (*http.Response, error) {
			if numCalls == 0 {
				return httpmock.NewStringResponse(200, `{"transient":{"cluster":{"routing":{"rebalance":{"enable":"all"}}}}}`), nil
			}
			return httpmock.NewStringResponse(200, string(intermediateClusterSettings)), nil
		})
	httpmock.RegisterResponder("PUT", "http://elasticsearch:9200/_cluster/settings",
		func(request *http.Request) (*http.Response, error) {
			var esSettings ESSettings
			bodyReader, _ := request.GetBody()
			_ = json.NewDecoder(bodyReader).Decode(&esSettings)

			if numCalls == 0 {
				bodyReader, _ = request.GetBody()
				intermediateClusterSettings, _ = ioutil.ReadAll(bodyReader)
			}

			if esSettings.GetTransientExcludeIPs().ValueOrZero() != "" || esSettings.GetTransientRebalance().ValueOrZero() != "" {
				return httpmock.NewStringResponse(400, ""), nil
			}

			if esSettings.GetPersistentExcludeIPs().ValueOrZero() != expectedSequenceOfExcludeIPs[numCalls] || esSettings.GetPersistentRebalance().ValueOrZero() != "none" {
				return httpmock.NewStringResponse(400, ""), nil
			}

			numCalls = numCalls + 1
			return httpmock.NewStringResponse(200, `{}`), nil
		})
	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/health",
		httpmock.NewStringResponder(200, `{"status":"green"}`))
	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/shards",
		httpmock.NewStringResponder(200, `[{"index":"a","ip":"10.2.19.5"},{"index":"b","ip":"10.2.10.2"},{"index":"c","ip":"10.2.16.2"}]`))

	esUrl, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint: esUrl,
	}
	err := systemUnderTest.Drain(context.TODO(), &v1.Pod{
		Status: v1.PodStatus{
			PodIP: "1.2.3.4",
		},
	})

	assert.NoError(t, err)

	info := httpmock.GetCallCountInfo()
	require.EqualValues(t, 1, info["GET http://elasticsearch:9200/_cluster/health"])
	require.EqualValues(t, 2, info["PUT http://elasticsearch:9200/_cluster/settings"])
	require.EqualValues(t, 2, info["GET http://elasticsearch:9200/_cluster/settings"])
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

	err := systemUnderTest.Cleanup(context.TODO())

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

	require.EqualValues(t, 3, len(indices), indices)
	require.EqualValues(t, "a", indices[0].Index, indices)
	require.EqualValues(t, 2, indices[0].Primaries, indices)
	require.EqualValues(t, 1, indices[0].Replicas, indices)

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

func TestUpdateIndexSettingsIgnoresUnknownIndex(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cluster/health",
		httpmock.NewStringResponder(200, `{"status":"green"}`))
	httpmock.RegisterResponder("PUT", "http://elasticsearch:9200/myindex/_settings",
		httpmock.NewStringResponder(404, `{}`))

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
	info := httpmock.GetCallCountInfo()

	assert.NoError(t, err)
	require.EqualValues(t, 1, info["PUT http://elasticsearch:9200/myindex/_settings"])
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

func TestExcludeSystemIndices(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://elasticsearch:9200/_cat/indices",
		httpmock.NewStringResponder(200, `[{"index":".system","pri":"1","rep":"1"},{"index":"a","pri":"1","rep":"1"}]`))

	url, _ := url.Parse("http://elasticsearch:9200")
	systemUnderTest := &ESClient{
		Endpoint:             url,
		excludeSystemIndices: true,
	}
	indices, err := systemUnderTest.GetIndices()

	assert.NoError(t, err)
	assert.Equal(t, 1, len(indices), indices)
	assert.Equal(t, "a", indices[0].Index, indices)

}

func TestESSettingsMergeNonEmtpyTransientSettings(t *testing.T) {
	type fields struct {
		Transient  ClusterSettings
		Persistent ClusterSettings
	}
	tests := []struct {
		name     string
		fields   fields
		expected ESSettings
	}{
		{
			name: "null transient settings should remain as null persistent settings",
			fields: fields{
				Transient: ClusterSettings{Cluster{Routing{
					Rebalance:  Rebalance{Enable: null.StringFromPtr(nil)},
					Allocation: Allocation{Exclude{IP: null.StringFromPtr(nil)}},
				}}},
			},
			expected: ESSettings{
				Transient: ClusterSettings{Cluster{Routing{
					Rebalance:  Rebalance{Enable: null.StringFromPtr(nil)},
					Allocation: Allocation{Exclude{IP: null.StringFromPtr(nil)}},
				}}},
				Persistent: ClusterSettings{Cluster{Routing{
					Rebalance:  Rebalance{Enable: null.StringFromPtr(nil)},
					Allocation: Allocation{Exclude{IP: null.StringFromPtr(nil)}},
				}}},
			},
		},
		{
			name: "copy over non empty transient cluster rebalance settings",
			fields: fields{
				Transient: ClusterSettings{Cluster{Routing{Rebalance: Rebalance{Enable: null.StringFrom("none")}}}},
			},
			expected: ESSettings{
				Persistent: ClusterSettings{Cluster{Routing{Rebalance: Rebalance{Enable: null.StringFrom("none")}}}},
			},
		},
		{
			name: "copy over non empty transient exclude ips string",
			fields: fields{
				Transient: ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("1.2.3.4")}}}}},
			},
			expected: ESSettings{
				Persistent: ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("1.2.3.4")}}}}},
			},
		},
		{
			name: "overwrite empty transient exclude ips string to null",
			fields: fields{
				Transient: ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("")}}}}},
			},
			expected: ESSettings{},
		},
		{
			name: "merge existing persistent exclude ips with transient exclude ips",
			fields: fields{
				Transient:  ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("1.2.3.4")}}}}},
				Persistent: ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("11.21.31.41")}}}}},
			},
			expected: ESSettings{
				Persistent: ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("1.2.3.4,11.21.31.41")}}}}},
			},
		},
		{
			name: "deduplicate transient exclude ips",
			fields: fields{
				Transient:  ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("1.2.3.4,1.2.3.4")}}}}},
				Persistent: ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("11.21.31.41")}}}}},
			},
			expected: ESSettings{
				Persistent: ClusterSettings{Cluster{Routing{Allocation: Allocation{Exclude{IP: null.StringFrom("1.2.3.4,11.21.31.41")}}}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esSettings := &ESSettings{
				Transient:  tt.fields.Transient,
				Persistent: tt.fields.Persistent,
			}
			esSettings.MergeNonEmptyTransientSettings()
			assert.Equal(t, tt.expected.GetPersistentRebalance(), esSettings.GetPersistentRebalance())
			assert.Equal(t, tt.expected.GetTransientRebalance(), esSettings.GetTransientRebalance())
			assert.Equal(t, tt.expected.GetPersistentExcludeIPs(), esSettings.GetPersistentExcludeIPs())
			assert.Equal(t, tt.expected.GetTransientExcludeIPs(), esSettings.GetTransientExcludeIPs())
		})
	}
}
