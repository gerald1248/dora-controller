package main

import (
	"testing"
)

func TestGetDeploymentFlags(t *testing.T) {
	var tests = []struct {
		description     string
		success         bool
		previousSuccess bool
		imageChanged    bool
		flag            string
	}{
		{"success", true, true, true, "DORA_SUCCESS|DORA_NEW_IMAGE|DORA_PREVIOUS_SUCCESS"},
		{"recovery", true, false, false, "DORA_RECOVERY|DORA_SAME_IMAGE|DORA_PREVIOUS_FAILURE"},
		{"failure", false, true, false, "DORA_FAILURE|DORA_SAME_IMAGE|DORA_PREVIOUS_SUCCESS"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			s := getDeploymentFlags(test.success, test.previousSuccess, test.imageChanged)
			if s != test.flag {
				t.Errorf("Unexpected flag %s for parameters success=%t, previousSuccess=%t, imageChanged=%t; expected %s", s, test.success, test.previousSuccess, test.imageChanged, test.flag)
			}
		})
	}
}
