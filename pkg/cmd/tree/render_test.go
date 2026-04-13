package tree

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSummarizeStatus(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]interface{}
		want string
	}{
		{
			name: "pod phase",
			obj: map[string]interface{}{
				"status": map[string]interface{}{"phase": "Running"},
			},
			want: "Running",
		},
		{
			name: "deployment replicas",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"replicas":      int64(3),
					"readyReplicas": int64(3),
				},
			},
			want: "3/3",
		},
		{
			name: "deployment partial ready",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"replicas":      int64(3),
					"readyReplicas": int64(1),
				},
			},
			want: "1/3",
		},
		{
			name: "service type and clusterIP",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type":      "ClusterIP",
					"clusterIP": "10.96.0.1",
				},
			},
			want: "ClusterIP 10.96.0.1",
		},
		{
			name: "configmap data keys",
			obj: map[string]interface{}{
				"data": map[string]interface{}{
					"key1": "val1",
					"key2": "val2",
				},
			},
			want: "2 keys",
		},
		{
			name: "empty object",
			obj:  map[string]interface{}{},
			want: "",
		},
		{
			name: "nil object",
			obj:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj *unstructured.Unstructured
			if tt.obj != nil {
				obj = &unstructured.Unstructured{Object: tt.obj}
			}
			got := summarizeStatus(obj)
			if got != tt.want {
				t.Errorf("summarizeStatus: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatNode(t *testing.T) {
	node := &TreeNode{
		GVK:  schema.GroupVersionKind{Kind: "Deployment"},
		Name: "nginx",
		Object: &unstructured.Unstructured{Object: map[string]interface{}{
			"status": map[string]interface{}{"phase": "Active"},
			"metadata": map[string]interface{}{
				"creationTimestamp": time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
			},
		}},
	}

	result := formatNode(node)
	if !strings.HasPrefix(result, "Deployment/nginx") {
		t.Errorf("expected prefix 'Deployment/nginx', got: %s", result)
	}
	if !strings.Contains(result, "Active") {
		t.Errorf("expected 'Active' in output, got: %s", result)
	}
	if !strings.Contains(result, "10m") {
		t.Errorf("expected age ~'10m' in output, got: %s", result)
	}
}

func TestRenderTree(t *testing.T) {
	root := &TreeNode{
		GVK:  schema.GroupVersionKind{Kind: "Deployment"},
		Name: "nginx",
		Object: &unstructured.Unstructured{Object: map[string]interface{}{
			"status": map[string]interface{}{"replicas": int64(2), "readyReplicas": int64(2)},
		}},
		Children: []*TreeNode{
			{
				GVK:  schema.GroupVersionKind{Kind: "ReplicaSet"},
				Name: "nginx-abc",
				Object: &unstructured.Unstructured{Object: map[string]interface{}{
					"status": map[string]interface{}{"replicas": int64(2), "readyReplicas": int64(2)},
				}},
				Children: []*TreeNode{
					{
						GVK:  schema.GroupVersionKind{Kind: "Pod"},
						Name: "nginx-abc-xx",
						Object: &unstructured.Unstructured{Object: map[string]interface{}{
							"status": map[string]interface{}{"phase": "Running"},
						}},
					},
					{
						GVK:  schema.GroupVersionKind{Kind: "Pod"},
						Name: "nginx-abc-yy",
						Object: &unstructured.Unstructured{Object: map[string]interface{}{
							"status": map[string]interface{}{"phase": "Running"},
						}},
					},
				},
			},
			{
				GVK:  schema.GroupVersionKind{Kind: "Service"},
				Name: "nginx-svc",
				Object: &unstructured.Unstructured{Object: map[string]interface{}{
					"spec": map[string]interface{}{"type": "ClusterIP", "clusterIP": "10.96.0.1"},
				}},
			},
		},
	}

	var buf bytes.Buffer
	RenderTree(root, &buf)
	output := buf.String()

	// Verify structure
	if !strings.Contains(output, "Deployment/nginx") {
		t.Errorf("missing root node in output:\n%s", output)
	}
	if !strings.Contains(output, "ReplicaSet/nginx-abc") {
		t.Errorf("missing replicaset in output:\n%s", output)
	}
	if !strings.Contains(output, "Pod/nginx-abc-xx") {
		t.Errorf("missing pod xx in output:\n%s", output)
	}
	if !strings.Contains(output, "Pod/nginx-abc-yy") {
		t.Errorf("missing pod yy in output:\n%s", output)
	}
	if !strings.Contains(output, "Service/nginx-svc") {
		t.Errorf("missing service in output:\n%s", output)
	}

	// Verify tree characters are present
	if !strings.Contains(output, "├") && !strings.Contains(output, "└") {
		t.Errorf("missing tree-drawing characters in output:\n%s", output)
	}
}
