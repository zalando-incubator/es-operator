package main

import "testing"

func TestLabelsString(t *testing.T) {
	labels := Labels(map[string]string{
		"master": "true",
	})
	expected := "master=true"

	if labels.String() != expected {
		t.Errorf("expected %s, got %s", expected, labels.String())
	}

}

func TestSetLabelsValue(t *testing.T) {
	for _, tc := range []struct {
		msg   string
		value string
		valid bool
	}{
		{
			msg:   "test valid labels",
			value: "master=true,worker=false",
			valid: true,
		},
		{
			msg:   "test invalid labels",
			value: "master=true=false",
			valid: false,
		},
	} {
		t.Run(tc.msg, func(t *testing.T) {
			labels := Labels(map[string]string{})
			err := labels.Set(tc.value)
			if err != nil && tc.valid {
				t.Errorf("should not fail: %s", err)
			}

			if err == nil && !tc.valid {
				t.Error("expected failure")
			}
		})
	}
}

func TestLabelsIsCumulative(t *testing.T) {
	var labels Labels
	if !labels.IsCumulative() {
		t.Error("expected IsCumulative = true")
	}
}
