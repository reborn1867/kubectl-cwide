package tree

import (
	"fmt"
	"io"
	"time"

	"github.com/xlab/treeprint"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
)

// RenderTree prints the tree to out using Unicode box-drawing characters.
// maxDepth <= 0 means unbounded. Cycles (revisits of the same UID) are broken
// with a "(cycle)" marker so the walk always terminates. When showNamespace is
// true (e.g. under --all-namespaces), each node is prefixed with its namespace
// so cross-namespace trees aren't ambiguous.
func RenderTree(root *TreeNode, out io.Writer, maxDepth int, showNamespace bool) {
	t := treeprint.NewWithRoot(formatNode(root, showNamespace))
	visited := map[types.UID]bool{root.UID: true}
	addChildren(t, root, visited, 1, maxDepth, showNamespace)
	fmt.Fprint(out, t.String())
}

func addChildren(branch treeprint.Tree, node *TreeNode, visited map[types.UID]bool, depth, maxDepth int, showNamespace bool) {
	if maxDepth > 0 && depth > maxDepth {
		if len(node.Children) > 0 {
			branch.AddNode(fmt.Sprintf("... (%d more, --max-depth=%d)", len(node.Children), maxDepth))
		}
		return
	}
	for _, child := range node.Children {
		if child.UID != "" && visited[child.UID] {
			branch.AddNode(formatNode(child, showNamespace) + "  (cycle)")
			continue
		}
		if child.UID != "" {
			visited[child.UID] = true
		}
		if len(child.Children) > 0 {
			sub := branch.AddBranch(formatNode(child, showNamespace))
			addChildren(sub, child, visited, depth+1, maxDepth, showNamespace)
		} else {
			branch.AddNode(formatNode(child, showNamespace))
		}
	}
}

// formatNode produces the display string for a tree line: Kind/name  status age.
// When showNamespace is true and the node is namespaced, the name is prefixed
// with "namespace/" so cross-namespace output is unambiguous.
func formatNode(node *TreeNode, showNamespace bool) string {
	kind := node.GVK.Kind
	status := summarizeStatus(node.Object)
	age := resourceAge(node.Object)

	name := node.Name
	if showNamespace && node.Namespace != "" {
		name = node.Namespace + "/" + node.Name
	}

	if status != "" && age != "" {
		return fmt.Sprintf("%s/%s  %s  %s", kind, name, status, age)
	}
	if status != "" {
		return fmt.Sprintf("%s/%s  %s", kind, name, status)
	}
	if age != "" {
		return fmt.Sprintf("%s/%s  %s", kind, name, age)
	}
	return fmt.Sprintf("%s/%s", kind, name)
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
