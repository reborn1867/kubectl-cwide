package tree

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/xlab/treeprint"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
)

// MetaDisplay controls whether each tree node appends its labels/annotations.
type MetaDisplay struct {
	ShowNamespace   bool
	ShowLabels      bool
	ShowAnnotations bool
}

// RenderTree prints the tree to out using Unicode box-drawing characters.
// maxDepth <= 0 means unbounded. Cycles (revisits of the same UID) are broken
// with a "(cycle)" marker so the walk always terminates. When md.ShowNamespace
// is true (e.g. under --all-namespaces), each node is prefixed with its
// namespace; ShowLabels/ShowAnnotations append the node's labels/annotations.
func RenderTree(root *TreeNode, out io.Writer, maxDepth int, md MetaDisplay) {
	t := treeprint.NewWithRoot(formatNode(root, md))
	visited := map[types.UID]bool{root.UID: true}
	addChildren(t, root, visited, 1, maxDepth, md)
	fmt.Fprint(out, t.String())
}

func addChildren(branch treeprint.Tree, node *TreeNode, visited map[types.UID]bool, depth, maxDepth int, md MetaDisplay) {
	if maxDepth > 0 && depth > maxDepth {
		if len(node.Children) > 0 {
			branch.AddNode(fmt.Sprintf("... (%d more, --max-depth=%d)", len(node.Children), maxDepth))
		}
		return
	}
	for _, child := range node.Children {
		if child.UID != "" && visited[child.UID] {
			branch.AddNode(formatNode(child, md) + "  (cycle)")
			continue
		}
		if child.UID != "" {
			visited[child.UID] = true
		}
		if len(child.Children) > 0 {
			sub := branch.AddBranch(formatNode(child, md))
			addChildren(sub, child, visited, depth+1, maxDepth, md)
		} else {
			branch.AddNode(formatNode(child, md))
		}
	}
}

// formatNode produces the display string for a tree line: Kind/name  status age.
// When md.ShowNamespace is true and the node is namespaced, the name is
// prefixed with "namespace/". ShowLabels/ShowAnnotations append the node's
// labels/annotations as sorted "k=v" pairs.
func formatNode(node *TreeNode, md MetaDisplay) string {
	kind := node.GVK.Kind
	status := summarizeStatus(node.Object)
	age := resourceAge(node.Object)

	name := node.Name
	if md.ShowNamespace && node.Namespace != "" {
		name = node.Namespace + "/" + node.Name
	}

	var line string
	switch {
	case status != "" && age != "":
		line = fmt.Sprintf("%s/%s  %s  %s", kind, name, status, age)
	case status != "":
		line = fmt.Sprintf("%s/%s  %s", kind, name, status)
	case age != "":
		line = fmt.Sprintf("%s/%s  %s", kind, name, age)
	default:
		line = fmt.Sprintf("%s/%s", kind, name)
	}

	if md.ShowLabels {
		line += "  labels=" + formatNodeMeta(node.Object, "labels")
	}
	if md.ShowAnnotations {
		line += "  annotations=" + formatNodeMeta(node.Object, "annotations")
	}
	return line
}

// formatNodeMeta renders a node's labels or annotations as sorted "k=v" pairs
// ("<none>" when empty), reading from the unstructured object's metadata.
func formatNodeMeta(obj *unstructured.Unstructured, kind string) string {
	if obj == nil {
		return "<none>"
	}
	var m map[string]string
	if kind == "annotations" {
		m = obj.GetAnnotations()
	} else {
		m = obj.GetLabels()
	}
	if len(m) == 0 {
		return "<none>"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, ",")
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
