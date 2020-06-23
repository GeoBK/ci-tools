package official

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/openshift/ci-tools/pkg/api"
)

func TestDefaultFields(t *testing.T) {
	var testCases = []struct {
		name  string
		input api.Release
		ouput api.Release
	}{
		{
			name: "nothing to do",
			input: api.Release{
				Architecture: api.ReleaseArchitectureAMD64,
				Channel:      api.ReleaseChannelStable,
				Version:      "4.4",
			},
			ouput: api.Release{
				Architecture: api.ReleaseArchitectureAMD64,
				Channel:      api.ReleaseChannelStable,
				Version:      "4.4",
			},
		},
		{
			name: "default architecture",
			input: api.Release{
				Channel: api.ReleaseChannelStable,
				Version: "4.4",
			},
			ouput: api.Release{
				Architecture: api.ReleaseArchitectureAMD64,
				Channel:      api.ReleaseChannelStable,
				Version:      "4.4",
			},
		},
	}

	for _, testCase := range testCases {
		actual, expected := defaultFields(testCase.input), testCase.ouput
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("%s: got incorrect candidate: %v", testCase.name, cmp.Diff(actual, expected))
		}
	}
}

func TestResolvePullSpec(t *testing.T) {
	var testCases = []struct {
		name        string
		release     api.Release
		raw         []byte
		expected    string
		expectedErr bool
	}{
		{
			name: "normal request",
			release: api.Release{
				Architecture: api.ReleaseArchitectureAMD64,
				Channel:      api.ReleaseChannelStable,
				Version:      "4.4",
			},
			raw:         []byte(`{"nodes":[{"version":"4.2.19","payload":"quay.io/openshift-release-dev/ocp-release@sha256:b51a0c316bb0c11686e6b038ec7c9f7ff96763f47a53c3443ac82e8c054bc035","metadata":{"description":"","io.openshift.upgrades.graph.previous.remove_regex":"4\\.1\\..*","io.openshift.upgrades.graph.release.channels":"candidate-4.2,fast-4.2,stable-4.2,candidate-4.3,fast-4.3,stable-4.3","io.openshift.upgrades.graph.release.manifestref":"sha256:b51a0c316bb0c11686e6b038ec7c9f7ff96763f47a53c3443ac82e8c054bc035","url":"https://access.redhat.com/errata/RHBA-2020:0460"}},{"version":"4.3.21","payload":"quay.io/openshift-release-dev/ocp-release@sha256:79a48030fc5e04fad0fd52f0cdd838ce94c7c1dfa7e7918fd7614d7bcab316f0","metadata":{"description":"","io.openshift.upgrades.graph.release.channels":"candidate-4.3,fast-4.3,stable-4.3,candidate-4.4,fast-4.4,stable-4.4","io.openshift.upgrades.graph.release.manifestref":"sha256:79a48030fc5e04fad0fd52f0cdd838ce94c7c1dfa7e7918fd7614d7bcab316f0","url":"https://access.redhat.com/errata/RHBA-2020:2129"}},{"version":"4.2.20","payload":"quay.io/openshift-release-dev/ocp-release@sha256:bd8aa8e0ce08002d4f8e73d6a2f9de5ae535a6a961ff6b8fdf2c52e4a14cc787","metadata":{"description":"","io.openshift.upgrades.graph.previous.remove_regex":"4\\.1\\..*","io.openshift.upgrades.graph.release.channels":"candidate-4.2,fast-4.2,stable-4.2,candidate-4.3,fast-4.3,stable-4.3","io.openshift.upgrades.graph.release.manifestref":"sha256:bd8aa8e0ce08002d4f8e73d6a2f9de5ae535a6a961ff6b8fdf2c52e4a14cc787","url":"https://access.redhat.com/errata/RHBA-2020:0523"}},{"version":"4.2.33","payload":"quay.io/openshift-release-dev/ocp-release@sha256:52e780ccc7e3af73b11dcb4afe275e2e743b59ccea6f228089ac93337de244d7","metadata":{"description":"","io.openshift.upgrades.graph.release.channels":"candidate-4.2,fast-4.2,stable-4.2,candidate-4.3,fast-4.3,stable-4.3","io.openshift.upgrades.graph.release.manifestref":"sha256:52e780ccc7e3af73b11dcb4afe275e2e743b59ccea6f228089ac93337de244d7","url":"https://access.redhat.com/errata/RHBA-2020:2023"}},{"version":"4.3.3","payload":"quay.io/openshift-release-dev/ocp-release@sha256:9b8708b67dd9b7720cb7ab3ed6d12c394f689cc8927df0e727c76809ab383f44","metadata":{"description":"","io.openshift.upgrades.graph.previous.remove_regex":".*","io.openshift.upgrades.graph.release.channels":"candidate-4.3,fast-4.3,stable-4.3","io.openshift.upgrades.graph.release.manifestref":"sha256:9b8708b67dd9b7720cb7ab3ed6d12c394f689cc8927df0e727c76809ab383f44","url":"https://access.redhat.com/errata/RHBA-2020:0528"}}]}`),
			expected:    "quay.io/openshift-release-dev/ocp-release@sha256:79a48030fc5e04fad0fd52f0cdd838ce94c7c1dfa7e7918fd7614d7bcab316f0",
			expectedErr: false,
		},
		{
			name: "malformed response errors",
			release: api.Release{
				Architecture: api.ReleaseArchitectureAMD64,
				Channel:      api.ReleaseChannelStable,
				Version:      "4.4",
			},
			raw:         []byte(`{"na1":}`),
			expected:    "",
			expectedErr: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept") != "application/json" {
					t.Error("did not get correct accept header")
					http.Error(w, "400 Bad Request", http.StatusBadRequest)
					return
				}
				if r.URL.Query().Get("channel") != fmt.Sprintf("%s-%s", testCase.release.Channel, testCase.release.Version) {
					t.Error("did not get correct channel query")
					http.Error(w, "400 Bad Request", http.StatusBadRequest)
					return
				}
				if r.URL.Query().Get("arch") != string(testCase.release.Architecture) {
					t.Error("did not get correct arch query")
					http.Error(w, "400 Bad Request", http.StatusBadRequest)
					return
				}
				if r.Method != http.MethodGet {
					t.Errorf("incorrect method to get a release: %s", r.Method)
					http.Error(w, "400 Bad Request", http.StatusBadRequest)
					return
				}
				if _, err := w.Write(testCase.raw); err != nil {
					t.Errorf("failed to write data: %v", err)
				}
			}))
			defer testServer.Close()
			actual, err := resolvePullSpec(testServer.URL, testCase.release)
			if err != nil && !testCase.expectedErr {
				t.Errorf("%s: expected no error but got one: %v", testCase.name, err)
			}
			if err == nil && testCase.expectedErr {
				t.Errorf("%s: expected an error but got none", testCase.name)
			}
			if actual != testCase.expected {
				t.Errorf("%s: got incorrect pullspec: %v", testCase.name, cmp.Diff(actual, testCase.expected))
			}
		})
	}
}

func TestLatestPullSpec(t *testing.T) {
	if actual := latestPullSpec([]Release{
		{Version: "4.2.19", Payload: "quay.io/openshift-release-dev/ocp-release@sha256:b51a0c316bb0c11686e6b038ec7c9f7ff96763f47a53c3443ac82e8c054bc035"},
		{Version: "4.3.21", Payload: "quay.io/openshift-release-dev/ocp-release@sha256:79a48030fc5e04fad0fd52f0cdd838ce94c7c1dfa7e7918fd7614d7bcab316f0"},
		{Version: "4.2.20", Payload: "quay.io/openshift-release-dev/ocp-release@sha256:bd8aa8e0ce08002d4f8e73d6a2f9de5ae535a6a961ff6b8fdf2c52e4a14cc787"},
	}); actual != "quay.io/openshift-release-dev/ocp-release@sha256:79a48030fc5e04fad0fd52f0cdd838ce94c7c1dfa7e7918fd7614d7bcab316f0" {
		t.Errorf("got incorrect latest pull-spec: %v", actual)
	}
}
