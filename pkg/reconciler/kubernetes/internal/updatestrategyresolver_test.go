package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest/fake"
	"k8s.io/kubectl/pkg/scheme"
)

type testCase struct {
	Name     string
	Response map[string]interface{}
	Resource *unstructured.Unstructured
	Want     UpdateStrategy
	WantErr  bool
}

func TestDefaultUpdateStrategyResolver_Resolve(t *testing.T) {

	testCases := []testCase{
		{
			Name:     "Pods should be created if missing",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"name":      "pod",
						"namespace": "kyma-system",
					},
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name: "Pods should be skipped",
			Response: map[string]interface{}{
				"kind":       "Pod",
				"apiVersion": "v1",
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"name":      "pod",
						"namespace": "kyma-system",
					},
				},
			},
			Want: SkipUpdateStrategy,
		},
		{
			Name:     "Jobs should be created if missing",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Job",
					"metadata": map[string]interface{}{
						"name":      "job",
						"namespace": "kyma-system",
					},
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name: "Jobs should be skipped",
			Response: map[string]interface{}{
				"kind":       "Job",
				"apiVersion": "batch/v1",
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Job",
					"metadata": map[string]interface{}{
						"name":      "job",
						"namespace": "kyma-system",
					},
				},
			},
			Want: SkipUpdateStrategy,
		},
		{
			Name:     "PVCs should be patched",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "PersistentVolumeClaim",
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name:     "ServiceAccounts should be patched",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ServiceAccount",
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name: "Statefulsets with PVC templates should be patched",
			Response: map[string]interface{}{
				"kind":       "StatefulSet",
				"apiVersion": "apps/v1",
				"spec": map[string]interface{}{
					"volumeClaimTemplates": []map[string]interface{}{
						{
							"kind": "PersistentVolumeClaim",
						},
					},
				},
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "postgresql",
						"namespace": "kyma-system",
					},
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name: "Statefulsets without PVC templates should be replaced",
			Response: map[string]interface{}{
				"kind":       "StatefulSet",
				"apiVersion": "apps/v1",
				"spec":       map[string]interface{}{},
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "postgresql2",
						"namespace": "kyma-system",
					},
				},
			},
			Want: ReplaceUpdateStrategy,
		},
		{
			Name:     "Anything else should be replaces",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Foooooo",
				},
			},
			Want: ReplaceUpdateStrategy,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			helper := newHelper(t, tc)
			d := newDefaultUpdateStrategyResolver(helper)
			got, err := d.Resolve(tc.Resource)
			if (err != nil) != tc.WantErr {
				t.Errorf("DefaultUpdateStrategyResolver.Resolve() error = %v, wantErr %v", err, tc.WantErr)
				return
			}
			if got != tc.Want {
				t.Errorf("DefaultUpdateStrategyResolver.Resolve() = %v, want %v", got, tc.Want)
			}
		})
	}
}

func newHelper(t *testing.T, tc testCase) *resource.Helper {
	httpClient := fake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet {
			if tc.Response == nil {
				return &http.Response{Body: nil, StatusCode: http.StatusNotFound, Header: header()}, nil
			}
			return createResponse(t, tc.Response), nil
		}
		return nil, fmt.Errorf("not supported method: %s", request.Method)
	})

	restClient := &fake.RESTClient{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client:               httpClient,
	}

	return &resource.Helper{
		RESTClient:      restClient,
		Resource:        "StatefulSet",
		NamespaceScoped: true,
	}
}

func createResponse(t *testing.T, responeContent map[string]interface{}) *http.Response {
	o := responeContent
	out, err := json.Marshal(o)
	require.NoError(t, err)
	reader := strings.NewReader(string(out))
	body := io.NopCloser(reader)
	resp := &http.Response{Body: body, StatusCode: http.StatusOK, Header: header()}
	return resp
}

func header() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}
