package main

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/ci-tools/pkg/api"
	"k8s.io/test-infra/prow/repoowners"
)

func TestGenerateMetadata(t *testing.T) {
	owners1 := repoowners.Config{
		Approvers: []string{"AlexNPavel", "stevekuznetsov"},
		Reviewers: []string{"AlexNPavel", "stevekuznetsov"},
	}
	owners2 := repoowners.Config{
		Approvers: []string{"petr-muller", "droslean"},
		Reviewers: []string{"petr-muller", "droslean"},
	}
	testCases := []struct {
		name             string
		regPath          string
		expectedMetadata api.RegistryMetadata
	}{{
		name:    "Registry",
		regPath: "../../test/multistage-registry/registry-metadata",
		expectedMetadata: api.RegistryMetadata{
			Metadata: map[string]api.RegistryInfo{
				"ipi-deprovision-chain.yaml": {
					Path:   "ipi/deprovision/ipi-deprovision-chain.yaml",
					Owners: owners2,
				},
				"ipi-deprovision-deprovision-ref.yaml": {
					Path:   "ipi/deprovision/deprovision/ipi-deprovision-deprovision-ref.yaml",
					Owners: owners2,
				},
				"ipi-deprovision-must-gather-ref.yaml": {
					Path:   "ipi/deprovision/must-gather/ipi-deprovision-must-gather-ref.yaml",
					Owners: owners2,
				},
				"ipi-install-chain.yaml": {
					Path:   "ipi/install/ipi-install-chain.yaml",
					Owners: owners1,
				},
				"ipi-install-install-ref.yaml": {
					Path:   "ipi/install/install/ipi-install-install-ref.yaml",
					Owners: owners1,
				},
				"ipi-install-rbac-ref.yaml": {
					Path:   "ipi/install/rbac/ipi-install-rbac-ref.yaml",
					Owners: owners1,
				},
				"ipi-workflow.yaml": {
					Path:   "ipi/ipi-workflow.yaml",
					Owners: owners1,
				},
			},
		},
	}}
	for _, testCase := range testCases {
		metadata, err := generateMetadata(testCase.regPath)
		if err != nil {
			t.Fatalf("%s: updateMetadata failed: %v", testCase.name, err)
		}
		if !reflect.DeepEqual(metadata, testCase.expectedMetadata) {
			t.Errorf("%s: Incorrect component path when updating metadata. Diff: %s", testCase.name, cmp.Diff(testCase.expectedMetadata, metadata))
		}
	}
}
