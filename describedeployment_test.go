package main

import (
	"github.com/acarl005/stripansi"
	"testing"
)

func TestDescribeDeployment(t *testing.T) {
	var tests = []struct {
		description string
		deployment  Deployment
		expected    string
	}{
		{"successful_deployment", Deployment{"server-c", "default", "openshift/hello-openshift", "DORA_SUCCESS|DORA_NEW_IMAGE|DORA_SUCCESSFUL_DEPLOYMENT|DORA_PREVIOUS_SUCCESS", 1561660013, 750000, true, true, 0, 0}, "DEBUG: Name=server-c  Namespace=default  Image=openshift/hello-openshift  ImageChanged=true  Flags=DORA_SUCCESS|DORA_NEW_IMAGE|DORA_SUCCESSFUL_DEPLOYMENT|DORA_PREVIOUS_SUCCESS  LastTimestamp=1561660013  RecoverySeconds=0  CommitTimestamp=750000  CycleTimeSeconds=0  Success=true"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			actual := stripansi.Strip(describeDeployment(test.deployment))
			if actual != test.expected {
				t.Errorf("Unexpected description '%s'; expected '%s'", actual, test.expected)
			}
		})
	}
}
