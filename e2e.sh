#!/bin/bash

# Simple script for running the e2e test from your local development machine
# against "some" cluster.
# You need to run `kubectl proxy` in a different terminal before running the
# script.
#
# If you want to test your local changes, build:
# IMAGE=es-operator VERSION=local make build.docker.local
# load it into kind:
# kind load docker-image es-operator:local
# .. and run the e2e tests with local image ref:
# IMAGE=es-operator:local ./e2e.sh

NAMESPACE="${NAMESPACE:-"es-operator-e2e-$(date +%s)"}"
IMAGE="${IMAGE:-"registry.opensource.zalan.do/pandora/es-operator:latest"}"
SERVICE_ENDPOINT_ES8="${SERVICE_ENDPOINT_ES8:-"http://127.0.0.1:8001/api/v1/namespaces/$NAMESPACE/services/es8-master:9200/proxy"}"
SERVICE_ENDPOINT_ES9="${SERVICE_ENDPOINT_ES9:-"http://127.0.0.1:8001/api/v1/namespaces/$NAMESPACE/services/es9-master:9200/proxy"}"
OPERATOR_ID="${OPERATOR_ID:-"e2e-tests"}"

# create namespace and resources
kubectl create ns "$NAMESPACE"
# apply CRDs (required for ElasticsearchDataSet resources)
kubectl apply -f docs/zalando.org_elasticsearchdatasets.yaml -f docs/zalando.org_elasticsearchmetricsets.yaml
kubectl --namespace "$NAMESPACE" apply -f cmd/e2e/account_cdp.yaml
# apply RBAC resources - first apply cluster-wide resources, then ServiceAccount to namespace
sed -e "s#{{{NAMESPACE}}}#$NAMESPACE#" < deploy/e2e/apply/rbac.yaml | kubectl apply -f -
# create ServiceAccount specifically in the target namespace
kubectl --namespace "$NAMESPACE" create serviceaccount es-operator --dry-run=client -o yaml | kubectl --namespace "$NAMESPACE" apply -f -
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es8-master.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es8-config.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es8-master-service.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es9-master.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es9-config.yaml
kubectl --namespace "$NAMESPACE" apply -f deploy/e2e/apply/es9-master-service.yaml
sed -e "s#{{{NAMESPACE}}}#$NAMESPACE#" \
    -e "s#{{{IMAGE}}}#$IMAGE#" \
    -e "s#{{{OPERATOR_ID}}}#$OPERATOR_ID#" < manifests/es-operator.yaml \
    | sed '/image: /{
a\
        imagePullPolicy: Never
}' \
    | kubectl --namespace "$NAMESPACE" apply -f -

# run e2e tests
ES_SERVICE_ENDPOINT_ES8=$SERVICE_ENDPOINT_ES8 \
    ES_SERVICE_ENDPOINT_ES9=$SERVICE_ENDPOINT_ES9 \
    E2E_NAMESPACE="$NAMESPACE" \
    OPERATOR_ID="$OPERATOR_ID" \
    KUBECONFIG=~/.kube/config echo go test -v -parallel 64 ./cmd/e2e/...

# kubectl delete ns "$NAMESPACE"
