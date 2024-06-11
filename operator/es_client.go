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
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/es-operator/operator/null"
	"gopkg.in/resty.v1"
	v1 "k8s.io/api/core/v1"
)

// ESClient is a pod drainer which can drain data from Elasticsearch pods.
type ESClient struct {
	Endpoint             *url.URL
	mux                  sync.Mutex
	excludeSystemIndices bool
	DrainingConfig       *DrainingConfig
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

type Exclude struct {
	IP null.String `json:"_ip,omitempty"`
}

type Allocation struct {
	Exclude Exclude `json:"exclude,omitempty"`
}
type Rebalance struct {
	Enable null.String `json:"enable,omitempty"`
}

type Routing struct {
	Allocation Allocation `json:"allocation,omitempty"`
	Rebalance  Rebalance  `json:"rebalance,omitempty"`
}

type Cluster struct {
	Routing Routing `json:"routing,omitempty"`
}

type ClusterSettings struct {
	Cluster Cluster `json:"cluster"`
}

// ESSettings represent response from _cluster/settings
type ESSettings struct {
	Transient  ClusterSettings `json:"transient,omitempty"`
	Persistent ClusterSettings `json:"persistent,omitempty"`
}

func deduplicateIPs(excludedIPsString string) string {
	if excludedIPsString == "" {
		return ""
	}

	uniqueIPsMap := make(map[string]struct{})
	uniqueIPsList := []string{}
	excludedIPs := strings.Split(excludedIPsString, ",")
	for _, excludedIP := range excludedIPs {
		if _, ok := uniqueIPsMap[excludedIP]; !ok {
			uniqueIPsMap[excludedIP] = struct{}{}
			uniqueIPsList = append(uniqueIPsList, excludedIP)
		}
	}

	return strings.Join(uniqueIPsList, ",")
}

func (esSettings *ESSettings) MergeNonEmptyTransientSettings() {
	if value := esSettings.GetTransientRebalance().ValueOrZero(); value != "" {
		esSettings.Persistent.Cluster.Routing.Rebalance.Enable = null.StringFromPtr(&value)
		esSettings.Transient.Cluster.Routing.Rebalance.Enable = null.StringFromPtr(nil)
	}

	transientExcludeIps := esSettings.GetTransientExcludeIPs().ValueOrZero()
	persistentExcludeIps := esSettings.GetPersistentExcludeIPs().ValueOrZero()
	if persistentExcludeIps == "" && transientExcludeIps != "" {
		esSettings.Persistent.Cluster.Routing.Allocation.Exclude.IP = null.StringFromPtr(&transientExcludeIps)
	} else if persistentExcludeIps != "" {
		uniqueIps := deduplicateIPs(mergeExcludeIpStrings(transientExcludeIps, persistentExcludeIps))
		mergedIps := null.StringFrom(uniqueIps)
		esSettings.Persistent.Cluster.Routing.Allocation.Exclude.IP = mergedIps
	}
	esSettings.Transient.Cluster.Routing.Allocation.Exclude.IP = null.StringFromPtr(nil)
}

func mergeExcludeIpStrings(transientExcludeIps string, persistentExcludeIps string) string {
	if transientExcludeIps == "" {
		return persistentExcludeIps
	}
	return transientExcludeIps + "," + persistentExcludeIps
}

func (esSettings *ESSettings) GetTransientExcludeIPs() null.String {
	return esSettings.Transient.Cluster.Routing.Allocation.Exclude.IP
}

func (esSettings *ESSettings) GetPersistentExcludeIPs() null.String {
	return esSettings.Persistent.Cluster.Routing.Allocation.Exclude.IP
}

func (esSettings *ESSettings) GetTransientRebalance() null.String {
	return esSettings.Transient.Cluster.Routing.Rebalance.Enable
}

func (esSettings *ESSettings) GetPersistentRebalance() null.String {
	return esSettings.Persistent.Cluster.Routing.Rebalance.Enable
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
	esSettings, err := c.getClusterSettings()
	if err != nil {
		return err
	}

	err = c.updateAutoRebalance("none", esSettings)
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

func (c *ESClient) Cleanup(_ context.Context) error {

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
	excludedIPsString := esSettings.GetPersistentExcludeIPs().ValueOrZero()
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
		err = c.setExcludeIPs(newExcludedIPsString, esSettings)
		if err != nil {
			return err
		}
	}

	if esSettings.GetPersistentRebalance().ValueOrZero() != "all" {
		c.logger().Info("Enabling auto-rebalance")
		return c.updateAutoRebalance("all", esSettings)
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
	esSettings.MergeNonEmptyTransientSettings()
	return &esSettings, nil
}

// adds the podIP to Elasticsearch exclude._ip list
func (c *ESClient) excludePodIP(pod *v1.Pod) error {

	c.mux.Lock()

	podIP := pod.Status.PodIP

	esSettings, err := c.getClusterSettings()
	if err != nil {
		c.mux.Unlock()
		return err
	}

	excludeString := esSettings.GetPersistentExcludeIPs().ValueOrZero()

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
		err = c.setExcludeIPs(strings.Join(ips, ","), esSettings)
	}

	c.mux.Unlock()
	return err
}

// undoExcludePodIP Removes the pod's IP from Elasticsearch exclude._ip list
func (c *ESClient) undoExcludePodIP(pod *v1.Pod) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	podIP := pod.Status.PodIP

	esSettings, err := c.getClusterSettings()
	if err != nil {
		return err
	}

	esExcludedIPsString := esSettings.GetPersistentExcludeIPs().ValueOrZero()
	if esExcludedIPsString == "" {
		// No excluded IPs, nothing to do
		return nil
	}

	esExcludedIPs := strings.Split(esExcludedIPsString, ",")
	var newESExcludedIPs []string

	for _, esExcludedIP := range esExcludedIPs {
		if esExcludedIP == podIP {
			// Skip the pod IP we want to remove
			continue
		}
		newESExcludedIPs = append(newESExcludedIPs, esExcludedIP)
	}

	// Sort and join the new list of excluded IPs
	sort.Strings(newESExcludedIPs)
	newESExcludedPodIPsString := strings.Join(newESExcludedIPs, ",")

	if newESExcludedPodIPsString == esExcludedIPsString {
		// No changes, so no update needed
		return nil
	}

	c.logger().Infof("Updating exclude._ip list to '%s' after removing IP '%s'", newESExcludedPodIPsString, podIP)

	// Update exclude._ip setting
	return c.setExcludeIPs(newESExcludedPodIPsString, esSettings)
}

func (c *ESClient) setExcludeIPs(ips string, originalESSettings *ESSettings) error {
	originalESSettings.updateExcludeIps(ips)
	resp, err := resty.New().R().
		SetHeader("Content-Type", "application/json").
		SetBody(originalESSettings).
		Put(c.Endpoint.String() + "/_cluster/settings")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	return nil
}

func (esSettings *ESSettings) updateExcludeIps(ips string) {
	esSettings.Persistent.Cluster.Routing.Allocation.Exclude.IP = null.StringFromPtr(&ips)
}

func (c *ESClient) updateAutoRebalance(value string, originalESSettings *ESSettings) error {
	originalESSettings.updateRebalance(value)
	resp, err := resty.New().R().
		SetHeader("Content-Type", "application/json").
		SetBody(originalESSettings).
		Put(c.Endpoint.String() + "/_cluster/settings")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("code status %d - %s", resp.StatusCode(), resp.Body())
	}
	return nil
}

func (esSettings *ESSettings) updateRebalance(value string) {
	esSettings.Persistent.Cluster.Routing.Rebalance.Enable = null.StringFromPtr(&value)
}

// repeatedly query shard allocations to ensure success of drain operation.
func (c *ESClient) waitForEmptyEsNode(ctx context.Context, pod *v1.Pod) error {
	podIP := pod.Status.PodIP
	_, err := resty.New().
		SetRetryCount(c.DrainingConfig.MaxRetries).
		SetRetryWaitTime(c.DrainingConfig.MinimumWaitTime).
		SetRetryMaxWaitTime(c.DrainingConfig.MaximumWaitTime).
		AddRetryCondition(
			// It is expected to return (bool, error) pair. Resty will retry
			// in case condition returns true or non nil error.
			func(r *resty.Response) (bool, error) {
				select {
				case <-ctx.Done():
					// Return false to not retry and return the context error directly.
					return false, ctx.Err()
				default:
					// Process response as normal if context is not done.
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

					// make sure the IP is still excluded, this could have been updated in the meantime.
					if remainingShards > 0 {
						err = c.excludePodIP(pod)
						if err != nil {
							return true, err
						}
					}
					return remainingShards > 0, nil
				}
			},
		).R().
		Get(c.Endpoint.String() + "/_cat/shards?h=index,ip&format=json")
	if err != nil {
		// If we were not able to finish the drain operation with success, remove the pod IP from the ES excluded IP list
		if undoErr := c.undoExcludePodIP(pod); undoErr != nil {
			return fmt.Errorf("failed to undo excluded pod IP: %v, original error: %v", undoErr, err)
		}
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

	returnStruct := make([]ESIndex, 0, len(esIndices))
	for _, index := range esIndices {
		// ignore system indices
		if c.excludeSystemIndices && strings.HasPrefix(index.Index, ".") {
			continue
		}
		// HACK: after-the-fact conversion of strings to integers from ES response.
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
			// if the index doesn't exist ES would return a 404
			if resp.StatusCode() == http.StatusNotFound {
				log.Warnf("Index '%s' not found, assuming it has been deleted.", index.Index)
				return nil
			}
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
				`{"settings": {"index" : {"number_of_replicas" : "%d", "number_of_shards": "%d", 
"routing.allocation.include.group": "%s"}}}`,
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
