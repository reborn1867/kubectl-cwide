package tree

import (
	"fmt"
	"io"
	"time"

	"github.com/xlab/treeprint"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/duration"
)

// RenderTree prints the tree to out using Unicode box-drawing characters.
func RenderTree(root *TreeNode, out io.Writer) {
	t := treeprint.NewWithRoot(formatNode(root))
	addChildren(t, root)
	fmt.Fprint(out, t.String())
}

func addChildren(branch treeprint.Tree, node *TreeNode) {
	for _, child := range node.Children {
		if len(child.Children) > 0 {
			sub := branch.AddBranch(formatNode(child))
			addChildren(sub, child)
		} else {
			branch.AddNode(formatNode(child))
		}
	}
}

// formatNode produces the display string for a tree line: Kind/name  status  age
func formatNode(node *TreeNode) string {
	kind := node.GVK.Kind
	status := summarizeStatus(node.Object)
	age := resourceAge(node.Object)

	if status != "" && age != "" {
		return fmt.Sprintf("%s/%s  %s  %s", kind, node.Name, status, age)
	}
	if status != "" {
		return fmt.Sprintf("%s/%s  %s", kind, node.Name, status)
	}
	if age != "" {
		return fmt.Sprintf("%s/%s  %s", kind, node.Name, age)
	}
	return fmt.Sprintf("%s/%s", kind, node.Name)
}

// summarizeStatus extracts a compact status string from the object.
func summarizeStatus(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	content := obj.UnstructuredContent()

	// Pods, Namespaces: .status.phase
	if phase, ok := nestedString(content, "status", "phase"); ok && phase != "" {
		return phase
	}

	// Deployments, ReplicaSets, StatefulSets: readyReplicas/replicas
	if replicas, ok := nestedInt64(content, "status", "replicas"); ok {
		ready, _ := nestedInt64(content, "status", "readyReplicas")
		return fmt.Sprintf("%d/%d", ready, replicas)
	}

	// Services: type + clusterIP
	if svcType, ok := nestedString(content, "spec", "type"); ok && svcType != "" {
		clusterIP, _ := nestedString(content, "spec", "clusterIP")
		if clusterIP != "" {
			return fmt.Sprintf("%s %s", svcType, clusterIP)
		}
		return svcType
	}

	// ConfigMaps, Secrets: number of data keys
	if data, ok := content["data"].(map[string]interface{}); ok {
		return fmt.Sprintf("%d keys", len(data))
	}

	return ""
}

// resourceAge computes the age from metadata.creationTimestamp.
func resourceAge(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	ts, ok := nestedString(obj.UnstructuredContent(), "metadata", "creationTimestamp")
	if !ok || ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	return duration.HumanDuration(time.Since(t))
}

// nestedString safely extracts a string value from a nested map.
func nestedString(obj map[string]interface{}, fields ...string) (string, bool) {
	cur := obj
	for i, f := range fields {
		if i == len(fields)-1 {
			v, ok := cur[f].(string)
			return v, ok
		}
		next, ok := cur[f].(map[string]interface{})
		if !ok {
			return "", false
		}
		cur = next
	}
	return "", false
}

// nestedInt64 safely extracts an int64 value from a nested map.
func nestedInt64(obj map[string]interface{}, fields ...string) (int64, bool) {
	cur := obj
	for i, f := range fields {
		if i == len(fields)-1 {
			switch v := cur[f].(type) {
			case int64:
				return v, true
			case float64:
				return int64(v), true
			default:
				return 0, false
			}
		}
		next, ok := cur[f].(map[string]interface{})
		if !ok {
			return 0, false
		}
		cur = next
	}
	return 0, false
}
