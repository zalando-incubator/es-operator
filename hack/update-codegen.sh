#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SRC="github.com"
GOPKG="$SRC/zalando-incubator/es-operator"
CUSTOM_RESOURCE_NAME="zalando.org"
CUSTOM_RESOURCE_VERSION="v1"

SCRIPT_ROOT="$(dirname ${BASH_SOURCE})/.."

# special setup for go modules
CODE_GEN_K8S_VERSION="c2090bec4d9b1fb25de3812f868accc2bc9ecbae" # 1.13.5 (https://github.com/kubernetes/code-generator/releases/tag/kubernetes-1.13.5)
packages=(
defaulter-gen
client-gen
lister-gen
informer-gen
deepcopy-gen
)
mkdir -p build
for pkg in "${packages[@]}"; do
    go get "k8s.io/code-generator/cmd/${pkg}@${CODE_GEN_K8S_VERSION}"
done
# use vendor/ as a temporary stash for code-generator.
rm -rf "${SCRIPT_ROOT}/vendor/k8s.io/code-generator"
rm -rf "${SCRIPT_ROOT}/vendor/k8s.io/gengo"
git clone --branch=release-1.12 https://github.com/kubernetes/code-generator.git "${SCRIPT_ROOT}/vendor/k8s.io/code-generator"
git clone https://github.com/kubernetes/gengo.git "${SCRIPT_ROOT}/vendor/k8s.io/gengo"

CODEGEN_PKG="${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}"

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.

OUTPUT_BASE="$(dirname ${BASH_SOURCE})/"

cd "${SCRIPT_ROOT}"
"${CODEGEN_PKG}/generate-groups.sh" all \
  "${GOPKG}/pkg/client" "${GOPKG}/pkg/apis" \
  "${CUSTOM_RESOURCE_NAME}:${CUSTOM_RESOURCE_VERSION}" \
  --go-header-file hack/boilerplate.go.txt \
  --output-base "$OUTPUT_BASE"

# To use your own boilerplate text append:
#   --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

# hack to make the generated code work with Go module based projects
cp -r "$OUTPUT_BASE/$GOPKG/pkg/apis" ./pkg
cp -r "$OUTPUT_BASE/$GOPKG/pkg/client" ./pkg
rm -rf "${OUTPUT_BASE:?}${SRC}"
