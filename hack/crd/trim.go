// This program removes descriptions from the input CRD yaml to reduce its size.
//
// # Why
//
// When CRD is applied via `kubectl apply -f docs/stackset_crd.yaml` (aka client-side apply) kubectl stores
// CRD content into the kubectl.kubernetes.io/last-applied-configuration annotation which has
// size limit of 262144 bytes.
// If the size of the annotation exceeds the limit, kubectl will fail with the following error:
//
//	The CustomResourceDefinition "stacksets.zalando.org" is invalid: metadata.annotations: Too long: must have at most 262144 bytes
//
// See https://github.com/kubernetes/kubectl/issues/712
//
// The CRD contains a lot of descriptions for k8s.io types and controller-gen
// does not allow to skip descriptions per field or per package,
// see https://github.com/kubernetes-sigs/controller-tools/issues/441
//
// # How
//
// It removes descriptions starting at the deepest level of the yaml tree
// until the size of the yaml converted to json is less than the maximum allowed annotation size.
package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sort"

	"sigs.k8s.io/yaml"
)

const maxAnnotationSize = 262144

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func mustGet[T any](v T, err error) T {
	must(err)
	return v
}

type description struct {
	depth    int
	property map[string]any
	value    string
}

func main() {
	yamlBytes := mustGet(io.ReadAll(os.Stdin))

	o := make(map[string]any)
	must(yaml.Unmarshal(yamlBytes, &o))

	jsonBytes := mustGet(json.Marshal(o))
	size := len(jsonBytes)

	descriptions := collect(o, 0)

	sort.Slice(descriptions, func(i, j int) bool {
		if descriptions[i].depth == descriptions[j].depth {
			return len(descriptions[i].value) > len(descriptions[j].value)
		}
		return descriptions[i].depth > descriptions[j].depth
	})

	for _, d := range descriptions {
		if size <= maxAnnotationSize {
			break
		}
		size -= len(d.value)
		delete(d.property, "description")
	}

	if size > maxAnnotationSize {
		log.Fatalf("YAML converted to JSON must be at most %d bytes long but it is %d bytes", maxAnnotationSize, size)
	}

	outYaml := mustGet(yaml.Marshal(o))
	mustGet(os.Stdout.Write(outYaml))
}

func collect(o any, depth int) (descriptions []description) {
	switch o := o.(type) {
	case map[string]any:
		for key, value := range o {
			switch value := value.(type) {
			case string:
				if key == "description" {
					descriptions = append(descriptions, description{depth, o, value})
				}
			default:
				descriptions = append(descriptions, collect(value, depth+1)...)
			}
		}
	case []any:
		for _, item := range o {
			descriptions = append(descriptions, collect(item, depth+1)...)
		}
	}
	return descriptions
}
