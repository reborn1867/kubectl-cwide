package tree

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/kubectl-cwide/pkg/models"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/jsonpath"
)

// ResolveChildren finds child resources matching the binding rule relative to the parent nodes.
// Returns a map from parent UID to the child TreeNodes belonging to that parent.
func ResolveChildren(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper meta.RESTMapper,
	relation models.TreeRelation,
	parentNodes []*TreeNode,
	namespace string,
) (map[types.UID][]*TreeNode, error) {
	switch relation.Bind.Type {
	case "ownerRef":
		return resolveByOwnerRef(ctx, dynClient, mapper, relation.Resource, parentNodes, namespace)
	case "labelSelector":
		return resolveByLabelSelector(ctx, dynClient, mapper, relation.Resource, parentNodes, namespace)
	case "fieldRef":
		return resolveByFieldRef(ctx, dynClient, mapper, relation.Resource, relation.Bind.Path, parentNodes, namespace)
	default:
		return nil, fmt.Errorf("unknown bind type %q", relation.Bind.Type)
	}
}

// resolveGVR converts a user-provided resource name (e.g. "replicasets") to a GroupVersionResource.
func resolveGVR(mapper meta.RESTMapper, resource string) (schema.GroupVersionResource, error) {
	return mapper.ResourceFor(schema.GroupVersionResource{Resource: resource})
}

// resolveByOwnerRef lists child resources and filters by ownerReference UID matching a parent.
func resolveByOwnerRef(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper meta.RESTMapper,
	childResource string,
	parentNodes []*TreeNode,
	namespace string,
) (map[types.UID][]*TreeNode, error) {
	gvr, err := resolveGVR(mapper, childResource)
	if err != nil {
		return nil, err
	}

	list, err := dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", childResource, err)
	}

	parentUIDs := make(map[types.UID]*TreeNode, len(parentNodes))
	for _, p := range parentNodes {
		parentUIDs[p.UID] = p
	}

	result := make(map[types.UID][]*TreeNode)
	for i := range list.Items {
		item := &list.Items[i]
		for _, ownerRef := range item.GetOwnerReferences() {
			if _, ok := parentUIDs[ownerRef.UID]; ok {
				node := nodeFromUnstructured(item)
				result[ownerRef.UID] = append(result[ownerRef.UID], node)
				break
			}
		}
	}
	return result, nil
}

// resolveByLabelSelector finds children by matching labels bidirectionally.
func resolveByLabelSelector(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper meta.RESTMapper,
	childResource string,
	parentNodes []*TreeNode,
	namespace string,
) (map[types.UID][]*TreeNode, error) {
	gvr, err := resolveGVR(mapper, childResource)
	if err != nil {
		return nil, err
	}

	result := make(map[types.UID][]*TreeNode)
	seen := make(map[string]bool) // track already-assigned child names per parent

	for _, parent := range parentNodes {
		parentLabels := extractLabels(parent.Object)
		if len(parentLabels) == 0 {
			continue
		}

		// Forward: use parent's selector labels to list children
		selector := labelsToSelector(parentLabels)
		list, err := dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list %s with selector %s: %w", childResource, selector, err)
		}

		for i := range list.Items {
			item := &list.Items[i]
			key := string(parent.UID) + "/" + item.GetName()
			if seen[key] {
				continue
			}
			seen[key] = true
			result[parent.UID] = append(result[parent.UID], nodeFromUnstructured(item))
		}

		// Reverse: list all children and check if child's spec.selector matches parent's pod labels
		podLabels := extractPodTemplateLabels(parent.Object)
		if len(podLabels) == 0 {
			continue
		}

		allList, err := dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue // non-fatal: forward match may have covered it
		}
		for i := range allList.Items {
			item := &allList.Items[i]
			key := string(parent.UID) + "/" + item.GetName()
			if seen[key] {
				continue
			}
			childSelector := extractSpecSelector(item)
			if len(childSelector) > 0 && isSubset(childSelector, podLabels) {
				seen[key] = true
				result[parent.UID] = append(result[parent.UID], nodeFromUnstructured(item))
			}
		}
	}

	return result, nil
}

// resolveByFieldRef extracts resource names from a parent's field using JSONPath, then fetches them.
func resolveByFieldRef(
	ctx context.Context,
	dynClient dynamic.Interface,
	mapper meta.RESTMapper,
	childResource string,
	path string,
	parentNodes []*TreeNode,
	namespace string,
) (map[types.UID][]*TreeNode, error) {
	gvr, err := resolveGVR(mapper, childResource)
	if err != nil {
		return nil, err
	}

	jpExpr, err := relaxedJSONPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid fieldRef path %q: %w", path, err)
	}

	jp := jsonpath.New("fieldRef").AllowMissingKeys(true)
	if err := jp.Parse(jpExpr); err != nil {
		return nil, fmt.Errorf("failed to parse JSONPath %q: %w", path, err)
	}

	result := make(map[types.UID][]*TreeNode)
	seen := make(map[string]bool)

	for _, parent := range parentNodes {
		names := extractJSONPathStrings(jp, parent.Object.UnstructuredContent())
		for _, name := range names {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true

			child, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue // resource may not exist
			}
			result[parent.UID] = append(result[parent.UID], nodeFromUnstructured(child))
		}
	}

	return result, nil
}

// nodeFromUnstructured creates a TreeNode from an Unstructured object.
func nodeFromUnstructured(obj *unstructured.Unstructured) *TreeNode {
	return &TreeNode{
		GVK:       obj.GetObjectKind().GroupVersionKind(),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		UID:       obj.GetUID(),
		Object:    obj,
	}
}

// extractLabels tries spec.selector.matchLabels, then spec.template.metadata.labels, then metadata.labels.
func extractLabels(obj *unstructured.Unstructured) map[string]string {
	content := obj.UnstructuredContent()

	// spec.selector.matchLabels
	if labels := nestedStringMap(content, "spec", "selector", "matchLabels"); len(labels) > 0 {
		return labels
	}
	// spec.template.metadata.labels
	if labels := nestedStringMap(content, "spec", "template", "metadata", "labels"); len(labels) > 0 {
		return labels
	}
	// metadata.labels
	if labels := nestedStringMap(content, "metadata", "labels"); len(labels) > 0 {
		return labels
	}
	return nil
}

// extractPodTemplateLabels returns spec.template.metadata.labels (the labels applied to pods).
func extractPodTemplateLabels(obj *unstructured.Unstructured) map[string]string {
	return nestedStringMap(obj.UnstructuredContent(), "spec", "template", "metadata", "labels")
}

// extractSpecSelector returns spec.selector as a flat map (for Services).
func extractSpecSelector(obj *unstructured.Unstructured) map[string]string {
	return nestedStringMap(obj.UnstructuredContent(), "spec", "selector")
}

// nestedStringMap extracts a map[string]string from nested fields.
func nestedStringMap(obj map[string]interface{}, fields ...string) map[string]string {
	cur := obj
	for i, f := range fields {
		if i == len(fields)-1 {
			raw, ok := cur[f].(map[string]interface{})
			if !ok {
				return nil
			}
			result := make(map[string]string, len(raw))
			for k, v := range raw {
				if s, ok := v.(string); ok {
					result[k] = s
				}
			}
			if len(result) == 0 {
				return nil
			}
			return result
		}
		next, ok := cur[f].(map[string]interface{})
		if !ok {
			return nil
		}
		cur = next
	}
	return nil
}

// labelsToSelector converts a label map to a comma-separated selector string.
func labelsToSelector(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

// isSubset returns true if all entries in sub exist in super.
func isSubset(sub, super map[string]string) bool {
	for k, v := range sub {
		if super[k] != v {
			return false
		}
	}
	return true
}

// relaxedJSONPath wraps a path in {. } if needed.
func relaxedJSONPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(path, "{") && strings.HasSuffix(path, "}") {
		return path, nil
	}
	if strings.HasPrefix(path, ".") {
		return "{" + path + "}", nil
	}
	return "{." + path + "}", nil
}

// extractJSONPathStrings evaluates a JSONPath and returns all string results.
func extractJSONPathStrings(jp *jsonpath.JSONPath, data interface{}) []string {
	results, err := jp.FindResults(data)
	if err != nil {
		return nil
	}
	var out []string
	for _, group := range results {
		for _, v := range group {
			s := fmt.Sprint(v.Interface())
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
	}
	return out
}

// extractJSONPathValues is an internal helper that flattens reflect.Value results.
func extractJSONPathValues(values [][]reflect.Value) []string {
	var out []string
	for _, group := range values {
		for _, v := range group {
			s := fmt.Sprint(v.Interface())
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
	}
	return out
}
