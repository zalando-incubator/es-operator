# Debugging HPA Resources and the Custom Metrics API Server

The [metrics server](https://github.com/kubernetes-sigs/metrics-server) is required to support the default
autoscaling [use cases](https://github.com/kubernetes-sigs/metrics-server#use-cases) like cpu/memory based scaling and
vertical scaling. In order to incorporate various other metrics like queue length, request traffic or disk space, a
custom monitoring system is required to collect the metrics and report them to the metric server or a [custom metrics
server](https://github.com/zalando-incubator/kube-metrics-adapter) can be implemented to support various other types of
metrics. Due to the standard [kubernetes metrics api](https://github.com/kubernetes/metrics), any component can collect
metrics and report them back to the metrics server as inputs to
the [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/).
The HPA aggregates the various reports and provides a single interface to manage
autoscaling [behavior](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/#autoscaling-on-multiple-metrics-and-custom-metrics)
.

The [Scale subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/_print/#scale-subresource)
has been defined to allow extending the HPA behavior to custom resource definitions which can adopt the consistent HPA
reconciliation to also support autoscaling. The ElasticsearchDataSets CRD also uses this subresource to incorporate
additional HPA features.

## Debugging the custom metrics server and custom scaling resources

The es-operator currently supports the ScalingSchedule and ClusterScalingSchedule custom scaling resources.
The [kube-metrics-adapter](https://github.com/zalando-incubator/kube-metrics-adapter) collects metrics from the scaling
schedule resources and reports them back to the configured [HPA](docs/hpa.yaml).

The customer metrics server can be first deployed and verified independently. The example
kube-metrics-adapter [manifest](manifest/kube-metrics-adapter.yaml) contains the RBAC and deployment resources for
handling the custom scaling schedule resources.

Deploy:

```
kubectl apply -f manifest/kube-metrics-adapter.yaml
```

Verify if the server is running:

```
kubectl -n kube-system get pods -l application=kube-metrics-adapter 
```

Verify if the custom metrics is working:

```
kubectl get --raw "/apis/custom.metrics.k8s.io/" | jq
```

The current version of the metrics adapter will return:

```
{
  "kind": "APIGroup",
  "apiVersion": "v1",
  "name": "custom.metrics.k8s.io",
  "versions": [
    {
      "groupVersion": "custom.metrics.k8s.io/v1beta2",
      "version": "v1beta2"
    },
    {
      "groupVersion": "custom.metrics.k8s.io/v1beta1",
      "version": "v1beta1"
    }
  ],
  "preferredVersion": {
    "groupVersion": "custom.metrics.k8s.io/v1beta2",
    "version": "v1beta2"
  }
}
```

A successful output indicates that the custom kubernetes metrics server is able to respond to the HPA. The output
doesn't indicate that the metrics collection is working. The server logs need to be consulted to verify any issues with
the metric collection itself.

More [examples](https://github.com/kubernetes-sigs/custom-metrics-apiserver) can be found in
the [custom-metrics-apiserver](https://github.com/kubernetes-sigs/custom-metrics-apiserver) repository.

## Verifying the Scale subresource on the EDS

The scale subresource attached to the EDS can be queried using the kubernetes api. The properties under the scale
subresource is set by the HPA and can be used to verify the hpa operations.

```
kubectl -n es-operator-demo get --raw /apis/zalando.org/v1/namespaces/es-operator-demo/elasticsearchdatasets/es-data-simple/scale | jq
```

For testing purposes any type of hpa metric can be used to verify the modification of the scale subresource. 


### References

- https://medium.com/@thescott111/autoscaling-kubernetes-custom-resource-using-the-hpa-957d00bb7993
- https://github.com/kubernetes-sigs/custom-metrics-apiserver
- https://github.com/kubernetes/metrics
- https://github.com/zalando-incubator/kube-metrics-adapter
- https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/_print/#scale-subresource