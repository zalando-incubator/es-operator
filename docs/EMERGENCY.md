# Emergency mode

ES-Operator will not operate a non-green cluster. Therefore, there are certain situations that require human intervention to recover an Elasticsearch cluster running with ES-Operator.

## Stopping the bleeding

The first and foremost goal should be to get your cluster back into green state. The [cluster reroute API](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-reroute.html#cluster-reroute-api-request-body) can be used to temporarily allocate an empty primary, until you have recovered your data.

If, for whatever reason, this is not feasible, you will need to switch from auto-pilot to manual steering. For this, please do the following:

## Manual Recovery Steps

1. Stop the ES-Operator.
2. If necessary, change the replicas in the stateful set attached to your EDS (it has the same name as the EDS) - **WARNING! Once the ES-Operator is started again, it will rigorously overwrite this setting, potentially leading to data loss by terminating pods without draining!**
3. After manual steering, in your EDS, disable auto-scaling (`spec.scaling.enabled: false`) and set `spec.replicas` to the current number of replicas in the STS. 
4. Start the ES-Operator again.
5. Re-enable auto-scaling.