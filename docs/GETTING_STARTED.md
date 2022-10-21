# Getting Started

This tutorial takes you step by step through deploying an Elasticsearch cluster managed by the ES Operator.

## Prerequisites

Have a Kubernetes cluster at hand (e.g. [kind](https://github.com/kubernetes-sigs/kind) or [minikube](https://github.com/kubernetes/minikube/)), and `kubectl` configured to point to it.

## Step 1 - Set up Roles

The ES Operator needs special permissions to access Kubernetes APIs, and Elasticsearch needs privileged access to increase the operating system limits for memory-mapped files.

Therefore as the first step we deploy a serviceAccount `operator` with the necessary RBAC roles attached.

```
kubectl apply -f docs/cluster-roles.yaml
```

If custom HPA metrics are needed, the kube-metrics-adapter must be deployed. The customer metrics server manifest contains the necessary roles and
the deployment for collecting and reporting from custom metric sources.

**Note: This is required if deploying to a local kubernetes setup. Please verify first if your is cluster already running a version of [custom metrics api server](https://github.com/kubernetes-sigs/custom-metrics-apiserver).**
```
kubectl apply -f manifests/kube-metrics-adapter.yaml
```

## Step 2 - Register Custom Resource Definitions

The ES Operator manages two custom resources. These need to be registered in your cluster.

```
kubectl apply -f docs/zalando.org_elasticsearchdatasets.yaml
kubectl apply -f docs/zalando.org_elasticsearchmetricsets.yaml
```

If the custom HPA metrics need to be tested or verified, add these additional resources to the cluster.

``` 
kubectl apply -f docs/zalando.org_scalingschedules.yaml
kubectl apply -f docs/zalando.org_clusterscalingschedules.yaml
```

## Step 3 - Deploy ES Operator

Next, we'll deploy our operator. It will be created from a deployment manifest in the namespace `es-operator-demo`, and pull the latest image.

```
kubectl apply -f docs/es-operator.yaml
```

You can check if it was successfully launched:

```
kubectl -n es-operator-demo get pods
```

## Step 4 - Bootstrap Your Elasticsearch Cluster

The Elasticsearch will be boot-strapped from a set of master nodes. For the purpose of this demo, a single master is sufficient. For production a set of three masters is recommended.

```
kubectl apply -f docs/elasticsearch-cluster.yaml
```

The manifest also creates services for the transport and HTTP protocols. If you tunnel to port 9200 on the master, you should be able to communicate with your Elasticsearch cluster.

```
MASTER_POD=$(kubectl -n es-operator-demo get pods -l application=elasticsearch,role=master -o custom-columns=:metadata.name --no-headers | head -n 1)
kubectl -n es-operator-demo port-forward $MASTER_POD 9200
```

## Step 5 - Add Elasticsearch Data Sets

Finally, let's add data nodes. For the purpose of this demo we have a simple stack will launch one data node, and has auto-scaling features turned off.

```
kubectl apply -f docs/elasticsearchdataset-simple.yaml
```

To check the results, first look for the custom resources.

```
kubectl -n es-operator-demo get eds
```

The ES Operator creates a StatefulSet, which will spawn the Pod. This can take a few minutes depending on your network and cluster performance.

```
kubectl -n es-operator-demo get sts
kubectl -n es-operator-demo get pods
```

## Step 6: Index Creation

We differentiated the stacks using an Elasticsearch node tag called `group`. It is advised to use this tag to bind indices with the same scaling requirements to nodes with the same `group` tag, by using the shard allocation setting like this:

 ```
curl -XPUT localhost:9200/demo-index -HContent-type:application/json \
 -d '{"settings": {"index": { "number_of_shards":5, "number_of_replicas":2, "routing.allocation.include.group": "group2"}}}'
 ```


## Step 7: Create HPA and ScalingSchedule manifests

The scaling schedule [manifest](docs/scaling_schedule.yaml) or other custom metric resources needs to be modified to fit the requirements. The customer metric must be added to the hpa [manifest](docs/hpa.yaml).
Deploy the scaling schedule and hpa manifest to start autoscaling on custom metrics.

```
kubectl apply -f docs/scaling_schedule.yaml
kubectl apply -f docs/hpa.yaml
```

Refer to the [DEBUGGING_HPA](docs/DEBUGGING_HPA) document for debugging the kube-metrics-adapter, HPA and custom metrics.

## Advanced Step: Auto-Scaling

Once you have gathered some experience in how the ES Operator handles your data nodes, you can start experimenting with auto-scaling features. The README.md offers some examples of different scaling scenarios, or look at the manifests of our [demo at the microXchg 2019](https://github.com/otrosien/microxchg19-demo).

## Advanced Step: Production-Readiness Features

If you understood how auto-scaling works, you can tackle the next steps towards production readiness.

The [Official Helm Charts](https://github.com/elastic/helm-charts/blob/master/elasticsearch/templates/statefulset.yaml) from Elasticsearch offer a few interesting features you may also want to integrate before going to production:

* Different persistence options
* Host-based anti-affinity
* Improved script-based readiness checks

We haven't seen it in their helm, but if you want high availability of your cluster, use the [allocation awareness](https://www.elastic.co/guide/en/elasticsearch/reference/current/allocation-awareness.html) features to ensure spread of the data across different locations, zones or racks.

## Advanced Step: Different Elasticsearch Clusters

Of course you can decide if the ES Operator should manage one big cluster, or you want to run multiple smaller clusters. This is totally possible. Just make sure they have different cluster names and use different hostnames for cluster discovery through the Kubernetes service.
