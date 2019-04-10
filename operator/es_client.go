package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// TODO make configurable as flags.
var (
	defaultRetryCount       = 999
	defaultRetryWaitTime    = 10 * time.Second
	defaultRetryMaxWaitTime = 30 * time.Second
)

// ESClient is a pod drainer which can drain data from Elasticsearch pods.
type ESClient struct {
	Endpoint *url.URL
}

// ESIndex represent an index to be used in public APIs
type ESIndex struct {
	Index     string `json:"index"`
	Primaries int32  `json:"pri"`
	Replicas  int32  `json:"rep"`
}

// ESShard represent a single shard from the response of _cat/shards
type ESShard struct {
	IP    string `json:"ip"`
	Index string `json:"index"`
}

// ESNode represent a single Elasticsearch node to be used in public API
type ESNode struct {
	IP              string  `json:"ip"`
	DiskUsedPercent float64 `json:"dup"`
}

// _ESIndex represent an index from the response of _cat/indices (only used internally!)
type _ESIndex struct {
	Index     string `json:"index"`
	Primaries string `json:"pri"`
	Replicas  string `json:"rep"`
}

// _ESNode represent a single Elasticsearch node from the response of _cat/nodes (only used internally)
type _ESNode struct {
	IP              string `json:"ip"`
	DiskUsedPercent string `json:"dup"`
}

type ESHealth struct {
	Status string `json:"status"`
}

// ESSettings represent response from _cluster/settings
type ESSettings struct {
	Transient struct {
		Cluster struct {
			Routing struct {
				Allocation struct {
					Exclude struct {
						IP string `json:"_ip"`
					} `json:"exclude"`
				} `json:"allocation"`
				Rebalance struct {
					Enable string `json:"enable"`
				} `json:"rebalance"`
			} `json:"routing"`
		} `json:"cluster"`
	} `json:"transient"`
}

func (c *ESClient) logger() *log.Entry {
	return log.WithFields(log.Fields{
		"endpoint": c.Endpoint,
	})
}

// Drain drains data from an Elasticsearch pod.
func (c *ESClient) Drain(ctx context.Context, pod *v1.Pod) error {

	c.logger().Info("Ensuring cluster is in green state")

	err := c.ensureGreenClusterState()
	if err != nil {
		return err
	}
	c.logger().Info("Disabling auto-rebalance")
	err = c.updateAutoRebalance("none")
	if err != nil {
		return err
	}
	c.logger().Infof("Excluding pod %s/%s from shard allocation", pod.Namespace, pod.Name)
	err = c.excludePodIP(pod)
	if err != nil {
		return err
	}

	c.logger().Info("Waiting for draining to finish")
	return c.waitForEmptyEsNode(ctx, pod)
}

func (c *ESClient) Cleanup(ctx context.Context) error {

	// 1. fetch IPs from _cat/nodes
	nodes, err := c.GetNodes()
	if err != nil {
		return err
	}

	// 2. fetch exclude._ip settings
	esSettings, err := c.getClusterSettings()
	if err != nil {
		return err
	}

	// 3. clean up exclude._ip settings based on known IPs from (1)
	excludedIPsString := esSettings.Transient.Cluster.Routing.Allocation.Exclude.IP
	excludedIPs := strings.Split(excludedIPsString, ",")
	var newExcludedIPs []string
	for _, excludeIP := range excludedIPs {
		for _, nodeIP := range nodes {
			if excludeIP == nodeIP.IP {
				newExcludedIPs = append(newExcludedIPs, excludeIP)
				sort.Strings(newExcludedIPs)
				break
			}
		}
	}

	newExcludedIPsString := strings.Join(newExcludedIPs, ",")
	if newExcludedIPsString != excludedIPsString {
		c.logger().Infof("Setting exclude list to '%s'", strings.Join(newExcludedIPs, ","))

		// 4. update exclude._ip setting
		err = c.setExcludeIPs(newExcludedIPsString)
		if err != nil {
			return err
		}
	}

	if esSettings.Transient.Cluster.Routing.Rebalance.Enable != "all" {
		c.logger().Info("Enabling auto-rebalance")
		return c.updateAutoRebalance("all")
	}
	return nil
}

// ensures cluster is in green state
func (c *ESClient) ensureGreenClusterState() error {
	resp, err := resty.New().R().
		Get(c.Endpoint.String() + "/_cluster/health?wait_for_status=green&timeout=60s")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	var esHealth ESHealth
	err = json.Unmarshal(resp.Body(), &esHealth)
	if err != nil {
		return err
	}
	if esHealth.Status != "green" {
		return fmt.Errorf("expected 'green', got '%s'", esHealth.Status)
	}
	return nil
}

// returns the response of the call to _cluster/settings
func (c *ESClient) getClusterSettings() (*ESSettings, error) {
	// get _cluster/settings for current exclude list
	resp, err := resty.New().R().
		Get(c.Endpoint.String() + "/_cluster/settings")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	var esSettings ESSettings
	err = json.Unmarshal(resp.Body(), &esSettings)
	if err != nil {
		return nil, err
	}
	return &esSettings, nil
}

// adds the podIP to Elasticsearch exclude._ip list
func (c *ESClient) excludePodIP(pod *v1.Pod) error {
	podIP := pod.Status.PodIP

	esSettings, err := c.getClusterSettings()
	if err != nil {
		return err
	}

	excludeString := esSettings.Transient.Cluster.Routing.Allocation.Exclude.IP

	// add pod IP to exclude list
	ips := []string{}
	if excludeString != "" {
		ips = strings.Split(excludeString, ",")
	}
	var foundPodIP bool
	for _, ip := range ips {
		if ip == podIP {
			foundPodIP = true
			break
		}
	}
	if !foundPodIP {
		ips = append(ips, podIP)
		sort.Strings(ips)
		return c.setExcludeIPs(strings.Join(ips, ","))
	}
	return nil
}

func (c *ESClient) setExcludeIPs(ips string) error {
	resp, err := resty.New().R().
		SetHeader("Content-Type", "application/json").
		SetBody([]byte(
			fmt.Sprintf(
				`{"transient" : {"cluster.routing.allocation.exclude._ip" : "%s"}}`,
				ips,
			),
		)).
		Put(c.Endpoint.String() + "/_cluster/settings")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	return nil
}

func (c *ESClient) updateAutoRebalance(value string) error {
	resp, err := resty.New().R().
		SetHeader("Content-Type", "application/json").
		SetBody([]byte(
			fmt.Sprintf(
				`{"transient" : {"cluster.routing.rebalance.enable" : "%s"}}`,
				value,
			),
		)).
		Put(c.Endpoint.String() + "/_cluster/settings")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	return nil
}

// repeatedly query shard allocations to ensure success of drain operation.
func (c *ESClient) waitForEmptyEsNode(ctx context.Context, pod *v1.Pod) error {
	// TODO: implement context handling
	podIP := pod.Status.PodIP
	_, err := resty.New().
		SetRetryCount(defaultRetryCount).
		SetRetryWaitTime(defaultRetryWaitTime).
		SetRetryMaxWaitTime(defaultRetryMaxWaitTime).
		AddRetryCondition(
			// It is expected to return (bool, error) pair. Resty will retry
			// in case condition returns true or non nil error.
			func(r *resty.Response) (bool, error) {
				var shards []ESShard
				err := json.Unmarshal(r.Body(), &shards)
				if err != nil {
					return true, err
				}
				// shardIP := make(map[string]bool)
				remainingShards := 0
				for _, shard := range shards {
					if shard.IP == podIP {
						remainingShards++
					}
				}
				c.logger().Infof("Found %d remaining shards on %s/%s (%s)", remainingShards, pod.Namespace, pod.Name, podIP)
				return remainingShards > 0, nil
			},
		).R().
		Get(c.Endpoint.String() + "/_cat/shards?h=index,ip&format=json")
	if err != nil {
		return err
	}
	return nil
}

func (c *ESClient) GetNodes() ([]ESNode, error) {
	resp, err := resty.New().R().
		Get(c.Endpoint.String() + "/_cat/nodes?h=ip,dup&format=json")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	var esNodes []_ESNode
	err = json.Unmarshal(resp.Body(), &esNodes)
	if err != nil {
		return nil, err
	}

	// HACK: after-the-fact conversion of string to float from ES response.
	returnStruct := make([]ESNode, 0, len(esNodes))
	for _, node := range esNodes {
		diskUsedPercent, err := strconv.ParseFloat(node.DiskUsedPercent, 64)
		if err != nil {
			if node.DiskUsedPercent != "" {
				c.logger().Warnf("Failed to parse '%s' as float for disk usage on '%s'. Falling back to 0", node.DiskUsedPercent, node.IP)
			}
			diskUsedPercent = 0
		}
		returnStruct = append(returnStruct, ESNode{
			IP:              node.IP,
			DiskUsedPercent: diskUsedPercent,
		})
	}

	return returnStruct, nil
}

func (c *ESClient) GetShards() ([]ESShard, error) {
	resp, err := resty.New().R().
		Get(c.Endpoint.String() + "/_cat/shards?h=index,ip&format=json")

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	var esShards []ESShard
	err = json.Unmarshal(resp.Body(), &esShards)
	if err != nil {
		return nil, err
	}
	return esShards, nil
}

func (c *ESClient) GetIndices() ([]ESIndex, error) {
	resp, err := resty.New().R().
		Get(c.Endpoint.String() + "/_cat/indices?h=index,pri,rep&format=json")

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	var esIndices []_ESIndex
	err = json.Unmarshal(resp.Body(), &esIndices)
	if err != nil {
		return nil, err
	}

	// HACK: after-the-fact conversion of strings to integers from ES response.
	returnStruct := make([]ESIndex, 0, len(esIndices))
	for _, index := range esIndices {
		primaries, err := strconv.Atoi(index.Primaries)
		if err != nil {
			return nil, err
		}
		replicas, err := strconv.Atoi(index.Replicas)
		if err != nil {
			return nil, err
		}
		returnStruct = append(returnStruct, ESIndex{
			Primaries: int32(primaries),
			Replicas:  int32(replicas),
			Index:     index.Index,
		})
	}

	return returnStruct, nil
}

func (c *ESClient) UpdateIndexSettings(indices []ESIndex) error {

	if len(indices) == 0 {
		return nil
	}

	err := c.ensureGreenClusterState()
	if err != nil {
		return err
	}

	for _, index := range indices {
		c.logger().Infof("Setting number_of_replicas for index '%s' to %d.", index.Index, index.Replicas)
		resp, err := resty.New().R().
			SetHeader("Content-Type", "application/json").
			SetBody([]byte(
				fmt.Sprintf(
					`{"index" : {"number_of_replicas" : "%d"}}`,
					index.Replicas,
				),
			)).
			Put(fmt.Sprintf("%s/%s/_settings", c.Endpoint.String(), index.Index))
		if err != nil {
			return err
		}

		if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
		}
	}
	return nil
}

func (c *ESClient) CreateIndex(indexName, groupName string, shards, replicas int) error {
	resp, err := resty.New().R().
		SetHeader("Content-Type", "application/json").
		SetBody([]byte(
			fmt.Sprintf(
				`{"index" : {"number_of_replicas" : "%d", "number_of_shards": "%d", 
"routing.allocation.include.group": "%s"}}`,
				replicas,
				shards,
				groupName,
			),
		)).
		Put(fmt.Sprintf("%s/%s", c.Endpoint.String(), indexName))
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	return nil
}

func (c *ESClient) DeleteIndex(indexName string) error {
	resp, err := resty.New().R().
		Delete(fmt.Sprintf("%s/%s", c.Endpoint.String(), indexName))
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	return nil
}
