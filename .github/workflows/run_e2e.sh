#!/usr/bin/env bash

ES_OPERATOR_IMAGE="${ES_OPERATOR_IMAGE:-"localhost:5000/es-operator:local"}"
namespace="es-operator-e2e"

# label nodes with lifecycle-status=ready label
for node in $(kind get nodes); do
    kubectl label node "${node}" --overwrite "lifecycle-status=ready"
done

kubectl proxy &
proxy_pid="$!"

# setup and run e2e
kubectl create ns "$namespace"
# deploy CRDs
kubectl apply -f docs/zalando.org_elasticsearchdatasets.yaml -f docs/zalando.org_elasticsearchmetricsets.yaml
kubectl apply -f docs/zalando.org_clusterscalingschedules.yaml -f docs/zalando.org_scalingschedules.yaml
# deploy sysctl ds
kubectl apply -f manifests/sysctl.yaml
# deploy metrics-server
kubectl apply -f manifests/metrics-server.yaml
# deploy kube-metrics-adapter
kubectl apply -f manifests/kube-metrics-adapter.yaml
# deploy manifests
for f in deploy/e2e/apply/*; do
  sed "s#{{{IMAGE}}}#${ES_OPERATOR_IMAGE}#" "$f" \
  | sed "s#{{{APPLICATION}}}#es-operator#" \
  | sed "s#{{{OPERATOR_ID}}}#es-operator-e2e#" \
  | sed "s#{{{NAMESPACE}}}#${namespace}#" | kubectl --namespace "$namespace" apply -f -
done
# wait for es-operator Pod to be ready
while [ "$(kubectl -n "$namespace" get pod -l application=es-operator -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}')" != "True" ]; do
    echo "Waiting for ready 'es-operator' pod"
    sleep 5
done
# wait for es master pods to be ready
for pod in es6-master-0 es7-master-0; do
    while [ "$(kubectl -n "$namespace" get pod "$pod" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')" != "True" ]; do
        echo "Waiting for ready '${pod}' pod"
        sleep 5
    done
done

# run e2e
OPERATOR_ID=es-operator-e2e \
E2E_NAMESPACE=es-operator-e2e \
ES_SERVICE_ENDPOINT_ES6="http://127.0.0.1:8001/api/v1/namespaces/es-operator-e2e/services/es6-master:9200/proxy" \
ES_SERVICE_ENDPOINT_ES7="http://127.0.0.1:8001/api/v1/namespaces/es-operator-e2e/services/es7-master:9200/proxy" \
KUBECONFIG="${HOME}/.kube/config" ./build/linux/e2e -test.v

kill "$proxy_pid"
