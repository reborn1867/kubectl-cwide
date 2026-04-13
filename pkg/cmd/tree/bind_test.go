package tree

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/jsonpath"
)

func makeNode(kind, name string, uid types.UID) *TreeNode {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
			"uid":       string(uid),
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: kind})
	obj.SetUID(uid)
	return &TreeNode{
		GVK:       schema.GroupVersionKind{Version: "v1", Kind: kind},
		Name:      name,
		Namespace: "default",
		UID:       uid,
		Object:    obj,
	}
}

func TestOwnerRefMatching(t *testing.T) {
	parent := makeNode("Deployment", "nginx", "uid-1")
	child := makeNode("ReplicaSet", "nginx-abc", "uid-2")
	child.Object.Object["metadata"].(map[string]interface{})["ownerReferences"] = []interface{}{
		map[string]interface{}{
			"uid":  "uid-1",
			"name": "nginx",
			"kind": "Deployment",
		},
	}

	refs := child.Object.GetOwnerReferences()
	if len(refs) == 0 {
		t.Fatal("expected ownerReferences on child")
	}
	if refs[0].UID != parent.UID {
		t.Errorf("ownerRef UID mismatch: got %q, want %q", refs[0].UID, parent.UID)
	}
}

func TestExtractLabels(t *testing.T) {
	tests := []struct {
		name   string
		obj    map[string]interface{}
		wantOK bool
		wantK  string
		wantV  string
	}{
		{
			name: "matchLabels",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "nginx",
						},
					},
				},
			},
			wantOK: true, wantK: "app", wantV: "nginx",
		},
		{
			name: "template labels fallback",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"tier": "frontend",
							},
						},
					},
				},
			},
			wantOK: true, wantK: "tier", wantV: "frontend",
		},
		{
			name: "metadata labels fallback",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"env": "prod",
					},
				},
			},
			wantOK: true, wantK: "env", wantV: "prod",
		},
		{
			name:   "no labels",
			obj:    map[string]interface{}{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: tt.obj}
			labels := extractLabels(u)
			if tt.wantOK {
				if labels == nil {
					t.Fatal("expected labels, got nil")
				}
				if labels[tt.wantK] != tt.wantV {
					t.Errorf("label %q: got %q, want %q", tt.wantK, labels[tt.wantK], tt.wantV)
				}
			} else {
				if labels != nil {
					t.Errorf("expected nil labels, got %v", labels)
				}
			}
		})
	}
}

func TestIsSubset(t *testing.T) {
	if !isSubset(map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}) {
		t.Error("expected subset")
	}
	if isSubset(map[string]string{"a": "1", "c": "3"}, map[string]string{"a": "1", "b": "2"}) {
		t.Error("expected not subset")
	}
	if !isSubset(map[string]string{}, map[string]string{"a": "1"}) {
		t.Error("empty is subset of anything")
	}
}

func TestRelaxedJSONPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{".spec.name", "{.spec.name}"},
		{"spec.name", "{.spec.name}"},
		{"{.spec.name}", "{.spec.name}"},
		{".spec.volumes[*].configMap.name", "{.spec.volumes[*].configMap.name}"},
	}
	for _, tt := range tests {
		got, err := relaxedJSONPath(tt.input)
		if err != nil {
			t.Errorf("relaxedJSONPath(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("relaxedJSONPath(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractJSONPathStrings(t *testing.T) {
	data := map[string]interface{}{
		"spec": map[string]interface{}{
			"volumes": []interface{}{
				map[string]interface{}{
					"configMap": map[string]interface{}{
						"name": "config-a",
					},
				},
				map[string]interface{}{
					"configMap": map[string]interface{}{
						"name": "config-b",
					},
				},
			},
		},
	}

	jp := jsonpath.New("test").AllowMissingKeys(true)
	if err := jp.Parse("{.spec.volumes[*].configMap.name}"); err != nil {
		t.Fatalf("failed to parse JSONPath: %v", err)
	}

	results := extractJSONPathStrings(jp, data)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
	if results[0] != "config-a" {
		t.Errorf("result[0]: got %q, want %q", results[0], "config-a")
	}
	if results[1] != "config-b" {
		t.Errorf("result[1]: got %q, want %q", results[1], "config-b")
	}
}
