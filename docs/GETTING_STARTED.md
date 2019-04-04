# Getting Started

This tutorial takes you step by step through deploying an Elasticsearch cluster managed by the ES Operator.

## Prerequisites

Have a Kubernetes cluster at hand (e.g. [kind](https://github.com/kubernetes-sigs/kind) or [minikube](https://github.com/kubernetes/minikube/)), and `kubectl` configured to point to it.

## Step 1 - Set up Roles

The ES Operator needs special permissions to access Kubernetes APIs, and Elasticsearch needs privileged access to increase the operating system limits for memory-mapped files.

Therefore as the first step we deploy a serviceAccount `operator` with the necessary RBAC roles attached.

```
kubectl apply -f cluster_roles.yaml
```

## Step 2 - Register Custom Resource Definitions

The ES Operator manages two custom resources. These need to be registered in your cluster.

```
kubectl apply -f custom_resource_definitions.yaml
```


## Step 3 - Deploy ES Operator

Next, we'll deploy our operator. It will be created from a deployment manifest, and pull the latest build.

```
kubectl apply -f es-operator.yaml
```

## Step 4 - Bootstrap Elasticsearch Cluster

The Elasticsearch will be boot-strapped from a set of master nodes. For the purpose of this demo, a single master is sufficient. For production a set of three masters is recommended.

The manifest also creates services for the transport and HTTP protocols. If you tunnel to port 9200 on the master, you should be able to communicate with your Elasticsearch cluster.

```
kubectl apply -f elasticsearch-cluster.yaml
```

## Step 5 - Add Elasticsearch Data Sets

Finally, let's add data nodes. There are two manifests for the purpose of this demo. The simple stack will launch one data node, and has auto-scaling off. There is also a scaling stack in case you want to experiment with auto-scaling settings.

```
kubectl apply -f elasticsearchdataset-simple.yaml
kubectl apply -f elasticsearchdataset-scaling.yaml
```

