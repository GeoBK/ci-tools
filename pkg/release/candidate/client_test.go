package candidate

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/openshift/ci-tools/pkg/api"
)

func TestServiceHost(t *testing.T) {
	var testCases = []struct {
		product      api.ReleaseProduct
		architecture api.ReleaseArchitecture
		ouput        string
	}{
		{
			product:      api.ReleaseProductOKD,
			architecture: api.ReleaseArchitectureAMD64,
			ouput:        "https://origin-release.svc.ci.openshift.org/api/v1/releasestream",
		},
		{

			product:      api.ReleaseProductOCP,
			architecture: api.ReleaseArchitectureAMD64,
			ouput:        "https://openshift-release.svc.ci.openshift.org/api/v1/releasestream",
		},
		{

			product:      api.ReleaseProductOCP,
			architecture: api.ReleaseArchitectureAMD64,
			ouput:        "https://openshift-release.svc.ci.openshift.org/api/v1/releasestream",
		},
		{

			product:      api.ReleaseProductOCP,
			architecture: api.ReleaseArchitecturePPC64le,
			ouput:        "https://openshift-release-ppc64le.svc.ci.openshift.org/api/v1/releasestream",
		},
		{

			product:      api.ReleaseProductOCP,
			architecture: api.ReleaseArchitectureS390x,
			ouput:        "https://openshift-release-s390x.svc.ci.openshift.org/api/v1/releasestream",
		},
	}

	for _, testCase := range testCases {
		if actual, expected := ServiceHost(testCase.product, testCase.architecture), testCase.ouput; actual != expected {
			t.Errorf("got incorrect service host: %v", cmp.Diff(actual, expected))
		}
	}
}

func TestEndpoint(t *testing.T) {
	var testCases = []struct {
		input api.Candidate
		ouput string
	}{
		{
			input: api.Candidate{
				Product:      api.ReleaseProductOKD,
				Architecture: api.ReleaseArchitectureAMD64,
				Stream:       api.ReleaseStreamOKD,
				Version:      "4.4",
			},
			ouput: "https://origin-release.svc.ci.openshift.org/api/v1/releasestream/4.4.0-0.okd/latest",
		},
		{
			input: api.Candidate{
				Product:      api.ReleaseProductOCP,
				Architecture: api.ReleaseArchitectureAMD64,
				Stream:       api.ReleaseStreamCI,
				Version:      "4.5",
			},
			ouput: "https://openshift-release.svc.ci.openshift.org/api/v1/releasestream/4.5.0-0.ci/latest",
		},
		{
			input: api.Candidate{
				Product:      api.ReleaseProductOCP,
				Architecture: api.ReleaseArchitectureAMD64,
				Stream:       api.ReleaseStreamNightly,
				Version:      "4.6",
			},
			ouput: "https://openshift-release.svc.ci.openshift.org/api/v1/releasestream/4.6.0-0.nightly/latest",
		},
		{
			input: api.Candidate{
				Product:      api.ReleaseProductOCP,
				Architecture: api.ReleaseArchitecturePPC64le,
				Stream:       api.ReleaseStreamCI,
				Version:      "4.7",
			},
			ouput: "https://openshift-release-ppc64le.svc.ci.openshift.org/api/v1/releasestream/4.7.0-0.ci-ppc64le/latest",
		},
		{
			input: api.Candidate{
				Product:      api.ReleaseProductOCP,
				Architecture: api.ReleaseArchitectureS390x,
				Stream:       api.ReleaseStreamNightly,
				Version:      "4.8",
			},
			ouput: "https://openshift-release-s390x.svc.ci.openshift.org/api/v1/releasestream/4.8.0-0.nightly-s390x/latest",
		},
	}

	for _, testCase := range testCases {
		if actual, expected := endpoint(testCase.input), testCase.ouput; actual != expected {
			t.Errorf("got incorrect endpoint: %v", cmp.Diff(actual, expected))
		}
	}
}

func TestDefaultFields(t *testing.T) {
	var testCases = []struct {
		name  string
		input api.Candidate
		ouput api.Candidate
	}{
		{
			name: "nothing to do",
			input: api.Candidate{
				Product:      api.ReleaseProductOKD,
				Architecture: api.ReleaseArchitectureAMD64,
				Stream:       api.ReleaseStreamOKD,
				Version:      "4.4",
			},
			ouput: api.Candidate{
				Product:      api.ReleaseProductOKD,
				Architecture: api.ReleaseArchitectureAMD64,
				Stream:       api.ReleaseStreamOKD,
				Version:      "4.4",
			},
		},
		{
			name: "default release stream for okd",
			input: api.Candidate{
				Product:      api.ReleaseProductOKD,
				Architecture: api.ReleaseArchitectureAMD64,
				Version:      "4.4",
			},
			ouput: api.Candidate{
				Product:      api.ReleaseProductOKD,
				Architecture: api.ReleaseArchitectureAMD64,
				Stream:       api.ReleaseStreamOKD,
				Version:      "4.4",
			},
		},
		{
			name: "default architecture",
			input: api.Candidate{
				Product: api.ReleaseProductOKD,
				Stream:  api.ReleaseStreamOKD,
				Version: "4.4",
			},
			ouput: api.Candidate{
				Product:      api.ReleaseProductOKD,
				Architecture: api.ReleaseArchitectureAMD64,
				Stream:       api.ReleaseStreamOKD,
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
		relative    int
		raw         []byte
		expected    string
		expectedErr bool
	}{
		{
			name:        "normal request",
			raw:         []byte(`{"name": "4.3.0-0.ci-2020-05-22-121811","phase": "Accepted","pullSpec": "registry.svc.ci.openshift.org/ocp/release:4.3.0-0.ci-2020-05-22-121811","downloadURL": "https://openshift-release-artifacts.svc.ci.openshift.org/4.3.0-0.ci-2020-05-22-121811"}`),
			expected:    "registry.svc.ci.openshift.org/ocp/release:4.3.0-0.ci-2020-05-22-121811",
			expectedErr: false,
		},
		{
			name:        "normal request with relative",
			relative:    10,
			raw:         []byte(`{"name": "4.3.0-0.ci-2020-05-22-121811","phase": "Accepted","pullSpec": "registry.svc.ci.openshift.org/ocp/release:4.3.0-0.ci-2020-05-22-121811","downloadURL": "https://openshift-release-artifacts.svc.ci.openshift.org/4.3.0-0.ci-2020-05-22-121811"}`),
			expected:    "registry.svc.ci.openshift.org/ocp/release:4.3.0-0.ci-2020-05-22-121811",
			expectedErr: false,
		},
		{
			name:        "malformed response errors",
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
				if r.Method != http.MethodGet {
					t.Errorf("incorrect method to get a release: %s", r.Method)
					http.Error(w, "400 Bad Request", http.StatusBadRequest)
					return
				}
				if testCase.relative != 0 {
					if relString := r.URL.Query().Get("rel"); relString != strconv.Itoa(testCase.relative) {
						t.Errorf("incorrect relative query param: %v", relString)
						http.Error(w, "400 Bad Request", http.StatusBadRequest)
						return
					}
				}
				if _, err := w.Write(testCase.raw); err != nil {
					t.Fatalf("http server Write failed: %v", err)
				}
			}))
			defer testServer.Close()
			actual, err := resolvePullSpec(testServer.URL, testCase.relative)
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
