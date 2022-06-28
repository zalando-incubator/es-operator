# Emergency mode

ES-Operator is not able to operate the cluster in a non-green [health status](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html#cluster-health-api-desc). Therefore, manual intervention might be required to recover the Elasticsearch cluster and re-enable ES-Operator to manage it.

## Stopping the bleeding

The first and foremost goal should be to get your cluster back into green health status. Opster lists a number of reasons for [cluster yellow](https://opster.com/guides/elasticsearch/operations/elasticsearch-yellow-status/) or [cluster red](https://opster.com/guides/elasticsearch/operations/elasticsearch-red-status/#:~:text=Overview,not%20right%20with%20the%20cluster), and how to recover from that.

One option to solve the health status problem would be to use the [cluster reroute API](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-reroute.html#cluster-reroute-api-request-body) to temporarily allocate an empty primary (`allocate_empty_primary`) until you have recovered your data. Other options include deleting the broken index or [recovering the index from a snapshot](https://www.elastic.co/guide/en/elasticsearch/reference/master/snapshot-restore-apis.html).

If, for whatever reason, this is not feasible in a timely manner, you will need to switch from auto-pilot to manual steering. For this, please do the following:

## Manual Recovery Steps

* Stop the ES-Operator, e.g.

    `kubectl scale --replicas=0 deployment/es-operator -n ${namespace}`

* If necessary, change the replicas in the stateful set (STS) attached to your EDS (it has the same name as the EDS), e.g.

    * `kubectl edit sts ${eds-name} -n ${namespace}`
        * set `spec.replicas` to the desired number

     **WARNING! Once the ES-Operator is started again, it will rigorously overwrite this setting, potentially leading to data loss by terminating pods without draining!**

* Fix the cluster issue, restoring its green health status

* Adjust your Elasticsearch data set (EDS) configuration, e.g.
    * `kubectl edit eds ${eds-name} -n ${namespace}`
        * set `spec.scaling.enabled: false`
        * set `spec.replicas` to the current number of replicas in the STS

* Start the ES-Operator, e.g.

    `kubectl scale --replicas=1 deployment/es-operator -n ${namespace}`

* Re-enable auto-scaling, e.g.
    * `kubectl edit eds ${eds-name} -n ${namespace}`
        * set `spec.scaling.enabled: true`
