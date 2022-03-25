# Migrating away from Transient Cluster Settings

The [transient cluster settings](https://www.elastic.co/guide/en/elasticsearch/reference/7.16/settings.html#cluster-setting-types)
are
being [deprecated](https://www.elastic.co/guide/en/elasticsearch/reference/7.16/migrating-7.16.html#breaking_716_settings_deprecations)
from ES 7.16.0. The es-operator was relying on the transient cluster settings for operating on the cluster because
transient settings have the highest priority. The es-operator controlled mainly the cluster rebalance and the exclude
ips list for the cluster scaling actions. Moving forward the es-operator will now exclusively operate only edit the
persistent cluster settings. Some teams might still rely on using the transient settings for manual cluster operations
which can inadvertently cause issues with the es-operator updates to the cluster settings causing an inconsistent
cluster state. To avoid this the es-operator is also copying any existing non empty transient settings related to the
cluster rebalance and the exclude ips list into the persistent settings before updating the new values for the
persistent settings. To avoid cluster inconsistencies with the new es-operator, we recommend the below migrations steps
before deploying the new es-operator.

1. Follow the
   official [Transient settings migration guide](https://www.elastic.co/guide/en/elasticsearch/reference/8.1/transient-settings-migration-guide.html).
2. Update any custom scripts that are still operating on transient settings.
3. Deploy the new es-operator.
