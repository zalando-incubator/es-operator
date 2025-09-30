#!/bin/bash

# Simple script for running the e2e test from your local development machine
# against "some" cluster.
# You need to run `kubectl proxy` in a different terminal before running the
# script.

NAMESPACE="${NAMESPACE:-"es-operator-e2e-$(date +%s)"}"
IMAGE="${IMAGE:-"registry.opensource.zalan.do/pandora/es-operator:latest"}"
SERVICE_ENDPOINT_ES8="${SERVICE_ENDPOINT_ES8:-"http://127.0.0.1:8001/api/v1/namespaces/$NAMESPACE/services/es8-master:9200/proxy"}"
SERVICE_ENDPOINT_ES7="${SERVICE_ENDPOINT_ES7:-"http://127.0.0.1:8001/api/v1/namespaces/$NAMESPACE/services/es7-master:9200/proxy"}"
OPERATOR_ID="${OPERATOR_ID:-"e2e-tests"}"

# create namespace and resources
kubectl create ns "$NAMESPACE"
kubectl --namespace "$NAMESPACE" apply -f cmd/e2e/account_cdp.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es8-master.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es8-config.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es8-master-service.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es7-master.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es7-config.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es7-master-service.yaml
sed -e "s#{{{NAMESPACE}}}#$NAMESPACE#" \
    -e "s#{{{IMAGE}}}#$IMAGE#" \
    -e "s#{{{OPERATOR_ID}}}#$OPERATOR_ID#" < manifests/es-operator.yaml \
    | kubectl --namespace "$NAMESPACE" apply -f -

# run e2e tests
ES_SERVICE_ENDPOINT_ES8=$SERVICE_ENDPOINT_ES8 \
    ES_SERVICE_ENDPOINT_ES7=$SERVICE_ENDPOINT_ES7 \
    E2E_NAMESPACE="$NAMESPACE" \
    OPERATOR_ID="$OPERATOR_ID" \
    KUBECONFIG=~/.kube/config go test -v -parallel 64 ./cmd/e2e/...

kubectl delete ns "$NAMESPACE"
