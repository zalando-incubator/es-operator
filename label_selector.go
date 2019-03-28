package main

import (
	"fmt"
	"strings"
)

// Labels is a map of labels.
type Labels map[string]string

func (l Labels) String() string {
	labels := make([]string, 0, len(l))
	for k, v := range l {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(labels, ",")
}

// Set parses a pod selector string and adds it to the list.
func (l Labels) Set(value string) error {
	labelsStrs := strings.Split(value, ",")
	for _, labelStr := range labelsStrs {
		kv := strings.Split(labelStr, "=")
		if len(kv) < 1 || len(kv) > 2 {
			return fmt.Errorf("invalid pod selector format")
		}

		val := ""
		if len(kv) == 2 {
			val = kv[1]
		}

		l[kv[0]] = val
	}

	return nil
}

// IsCumulative always return true because it's allowed to call Set multiple
// times.
func (l Labels) IsCumulative() bool {
	return true
}
