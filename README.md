# Elasticsearch Operator

[![Build Status](https://travis-ci.org/zalando-incubator/es-operator.svg?branch=master)](https://travis-ci.org/zalando-incubator/es-operator)
[![Coverage Status](https://coveralls.io/repos/github/zalando-incubator/es-operator/badge.svg?branch=master)](https://coveralls.io/github/zalando-incubator/es-operator?branch=master)
[![GitHub release](https://img.shields.io/github/release/zalando-incubator/es-operator.svg)](https://github.com/zalando-incubator/es-operator/releases)
[![go-doc](https://godoc.org/github.com/zalando-incubator/es-operator?status.svg)](https://godoc.org/github.com/zalando-incubator/es-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/zalando-incubator/es-operator)](https://goreportcard.com/report/github.com/zalando-incubator/es-operator)

This is an operator for running Elasticsearch in Kubernetes with focus on operational aspects, like safe draining and offering auto-scaling capabilities for Elasticsearch data nodes, rather than just abstracting manifest definitions.

## License

Starting with `v0.1.3` the ES-Operator is dual-licensed under [MIT](/LICENSE.MIT.txt) and [Apache-2.0](/LICENSE.Apache2.txt) license. You can choose between one of them if you use this work.

`SPDX-License-Identifier: MIT OR Apache-2.0`

## Compatibility

The ES-Operator has been tested with Elasticsearch 6.x and 7.x.

## How it works

The operator works by managing custom resources called `ElasticsearchDataSets` (EDS). They
are basically a thin wrapper around StatefulSets. One EDS represents a common group of Elasticsearch data nodes. When applying an EDS manifest the operator will create and manage a corresponding StatefulSet.

**Do not operate manually on the StatefulSet. The operator is supposed to own this resource on your behalf.**

## Key features

* It can scale in two dimensions, shards per node and number of replicas for the indices on that dataset.
* It works within scaling dimensions known to and long-term tested by teams in Zalando.
* Target CPU ratio is a safe and well-known metric to scale on in order to avoid latency spikes caused by Garbage Collection.
* In case of emergency, manual scaling is possible by disabling the auto-scaling feature.

## Getting Started

For a quick tutorial how to deploy the ES Operator look at our [Getting Started Guide](docs/GETTING_STARTED.md).

## Custom Resource

### Full Example

```
apiVersion: zalando.org/v1
kind: ElasticsearchDataSet
spec:
  replicas: 2
  skipDraining: false
  scaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 99
    minIndexReplicas: 2
    maxIndexReplicas: 3
    minShardsPerNode: 2
    maxShardsPerNode: 6
    scaleUpCPUBoundary: 50
    scaleUpThresholdDurationSeconds: 900
    scaleUpCooldownSeconds: 3600
    scaleDownCPUBoundary: 25
    scaleDownThresholdDurationSeconds: 1800
    scaleDownCooldownSeconds: 3600
    diskUsagePercentScaledownWatermark: 80
```

### Custom resource properties


| Key      | Description   | Type |
|----------|---------------|---------|
| spec.replicas | Initial size of the StatefulSet. If auto-scaling is disabled, this is your desired cluster size. | Int |
| spec.excludeSystemIndices | Enable or disable inclusion of system indices like '.kibana' when calculating shard-per-node ratio and scaling index replica counts. Those are usually managed by Elasticsearch internally. Default is false for backwards compatibility | Boolean |
| spec.skipDraining | Allows the ES Operator to terminate an Elasticsearch node without re-allocating its data. This is useful for persistent disk setups, like EBS volumes. Beware that the ES Operator does not verify that you have more than one copy of your indices and therefore wouldn't protect you from potential data loss. (default=false) | Boolean |
| spec.scaling.enabled | Enable or disable auto-scaling. May be necessary to enforce manual scaling. | Boolean |
| spec.scaling.minReplicas | Minimum Pod replicas. Lower bound (inclusive) when scaling down.  | Int |
| spec.scaling.maxReplicas | Maximum Pod replicas. Upper bound (inclusive) when scaling up.  | Int |
| spec.scaling.minIndexReplicas | Minimum index replicas. Lower bound (inclusive) when reducing index copies. (reminder: total copies is replicas+1 in Elasticsearch) | Int |
| spec.scaling.maxIndexReplicas | Maximum index replicas. Upper bound (inclusive) when increasing index copies.  | Int |
| spec.scaling.minShardsPerNode | Minimum shard per node ratio. When reached, scaling up also requires adding more index replicas.  | Int |
| spec.scaling.maxShardsPerNode | Maximum shard per node ratio. Boundary for scaling down. | Int |
| spec.scaling.scaleUpCPUBoundary | (Median) CPU consumption/request ratio to consistently exceed in order to trigger scale up. | Int |
| spec.scaling.scaleUpThresholdDurationSeconds | Duration in seconds required to meet the scale-up criteria before scaling. | Int |
| spec.scaling.scaleUpCooldownSeconds | Minimum duration in seconds between two scale up operations. | Int |
| spec.scaling.scaleDownCPUBoundary | (Median) CPU consumption/request ratio to consistently fall below in order to trigger scale down. | Int |
| spec.scaling.scaleDownThresholdDurationSeconds | Duration in seconds required to meet the scale-down criteria before scaling. | Int |
| spec.scaling.scaleDownCooldownSeconds | Minimum duration in seconds between two scale-down operations. | Int |
| spec.scaling.diskUsagePercentScaledownWatermark | If disk usage on one of the nodes exceeds this threshold, scaling down will be prevented. | Float |
| status.lastScaleUpStarted | Timestamp of start of last scale-up activity | Timestamp |
| status.lastScaleUpEnded | Timestamp of end of last scale-up activity | Timestamp |
| status.lastScaleDownStarted | Timestamp of start of last scale-down activity | Timestamp |
| status.lastScaleDownEnded | Timestamp of end of last scale-down activity | Timestamp |


## How it scales


The operator will collect the median CPU consumption from all Pods of the EDS every 60 seconds. Based
on the data it will decide if scale-up or scale-down is necessary. For this to
happen all samples within the given period need to meet the configured scaling threshold.

The actual calculation of how many resources to allocate is based on the
idea of managing the shard-per-node ratio inside the cluster. Scaling out decreases the
shard-to-node ratio, increasing available resources per index, while scaling in increases
the shard-to-node ratio. We rely on auto-rebalancing of Elasticsearch to ensure this ratio
is equally distributed among the nodes.

At a certain point it's not feasible to only add more nodes. This can be the case if
you already reached the lower bound of one shard per node. In other cases you may want to
increase concurrent capacity for an index. Consequently the operator is able to add index
replicas when scaling out, and removing them before scaling in again. All you need to do is define the upper and lower bound of shards per node.

## Example 1

* One index with 6 shards. minReplicas = 2, maxReplicas=4, minShardsPerNode=1, maxShardsPerNode=3, targetCPU: 40%
* initial, minimal deployment: 3 copies of index x 6 shards = 18 shards / 3-per-node => 6 nodes
* Mean cpu-utilization exceeds 40% for more than 20 minutes => scale-up. First by decreasing the shards-per-node ratio to 2: 18 shards / 2 per-node => 9 nodes
* Mean cpu-utilization exceeds 40% for more than 20 minutes => scale-up by decreasing shard-per-node ratio to 1: 18 shards / 1 per node => 18 nodes
* Mean cpu-utilization exceeds 40% for more than 20 minutes => scale-up by increasing replica count to 3: 24 shards / 1 per node => 24 nodes
* Mean cpu-utilization exceeds 40% for more than 20 minutes => scale-up by increasing replica count to 4: 36 shards / 1 per node => 36 nodes
* No more further scale-up (safety net to avoid cost explosion)
* Scale-down in reverse order. So, if expected average CPU utilization would be below 40%, e.g. current=20%, expected=20%/24*36=30% => decrease replica count to 3: 24 shards / 1 per node => 24 nodes
* etc....

## Example 2

* Four indices with 6 shards. minReplicas = 2, maxReplicas=3, minShardsPerNode=2, maxShardsPerNode=4, targetCPU: 40%
* Initial, minimal deployment: 3 copies x 4 indices x 6 shards = 72 shards / 4-per-node => 18 nodes
* Mean cpu-utilization exceeds 40% for more than 20 minutes => scale-up. First by decreasing the shards-per-node ratio to 3: 72 shards / 3 per-node => 24 nodes
* Mean cpu-utilization exceeds 40% for more than 20 minutes => scale-up by decreasing the shards-per-node ratio to 2: 72 shards / 2 per-node => 36 nodes
* Mean cpu-utilization exceeds 40% for more than 20 minutes => scale-up by increasing replicas. 4 copies x 4 indices x 6 shards = 96 shards / 2  per node => 48 nodes

## Scale-up operation

* If scale-up requires increase of replicas, disable shard-rebalancing
* Calculate required Pod count by retrieving the current indices, their shard counts and current replica vs. desired replica count counts.
* Scale up by updating `spec.Replicas` and start the resource reconciliation process.
* If scale-up requires increase of replicas, wait for the StatefulSet to stabilize before updating `index.number_of_replicas` on Elasticsearch.

## Scale-down operation

* Calculate required Pod count by retrieving the current indices, their shard count and current replica vs. desired replica count count.
* If scale-down requires decrease of replicas, update `index.number_of_replicas` on each index
* Scale down


## Draining and rolling restarts

The operator will poll for all managed Pods and determine if any of the Pods
needs to be drained/updated. It determines if updates are needed based on the
following logic and priority:

1. Pods already marked draining should be drained to completion and be deleted.
2. Pods on a priority node (e.g. a node about to be terminated) should be drained.
3. Pods not up to date with StatefulSet revision gets should be drained.

If multiple Pods needs to be updated the update is done based on the above
priority where '1' is the highest.


## What it does not do

The operator does not manage Elasticsearch master nodes. You can create them on your own, most likey using a standard deployment or a StatefulSet manifest.

## Building

This project uses [Go modules](https://github.com/golang/go/wiki/Modules) as
introduced in Go 1.11 therefore you need Go >=1.11 installed in order to build.
If using Go 1.11 you also need to [activate Module
support](https://github.com/golang/go/wiki/Modules#installing-and-activating-module-support).

Assuming Go has been setup with module support it can be built simply by running:

```sh
export GO111MODULE=on # needed if the project is checked out in your $GOPATH.
$ make
```


## Running

The `es-operator` can be run as a deployment in the cluster. See
[es-operator.yaml](/docs/es-operator.yaml) for an example.

By default the operator will manage all `ElasticsearchDataSets` in the cluster
but you can limit it to a certain resources by setting the `--operator-id`
and/or `--namespace` options.

When the operator is run with `--operator-id=my-operator` it will only manage
`ElasticseachDataSets` which has the following annotation set:

```yaml
metadata:
  annotations:
    es-operator.zalando.org/operator: my-operator
```

Operators which doesn't run with the `--operator-id` flag will only operate on
resources which doesn't have the annotation.

When it's run with `--namespace=my-namespace` it will only manage resources in
the `my-namespace` namespace.

Can be deployed just by running:

```bash
$ kubectl apply -f docs/es-operator.yaml
```

### Running locally

The operator can be run locally and operate on a remote cluster making it
simpler to iterate during development.

To run it locally you need to run `kubectl proxy` in one shell, and then you
can start the operator with the following flags:

```bash
$ ./build/es-operator \
  --priority-node-selector=lifecycle-status=ready \
  --apiserver=http://127.0.0.1:8001 \
  --operator-id=my-operator \
  --elasticsearch-endpoint=http://127.0.0.1:8001/api/v1/namespaces/default/services/elasticsearch:9200/proxy
```

This assumes that the `elasticsearch-endpoint` is exposed via a service running
in the `default` namespace. This uses the kube-apiserver proxy functionality to
proxy requests to the Elasticsearch cluster.

## Other alternatives

We are not the only ones providing an Elasticsearch operator for Kubernetes. Here are some alternatives you might want to look at.

* [upmc-enterprises/elasticsearch-operator](https://github.com/upmc-enterprises/elasticsearch-operator) - offers a higher level abstraction of the custom resource definition of an Elasticsearch cluster, snapshotting support, but to our knowledge no scaling support and no draining of nodes.
* [jetstack/navigator](https://github.com/jetstack/navigator) - operator that can handle both Cassandra and Elasticsearch clusters, but doesn't offer auto-scaling or draining of nodes.

