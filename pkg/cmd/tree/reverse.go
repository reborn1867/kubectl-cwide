package tree

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// runReverse walks upward from the given node via ownerReferences, producing a
// tree rooted at the topmost ancestor and rendered downward toward the caller's
// resource. Best-effort: if an ownerRef points to a Kind that doesn't map to
// a resource we can list, the chain stops there.
func (o *TreeOptions) runReverse(ctx context.Context, start *TreeNode) error {
	chain := []*TreeNode{start}
	visited := map[types.UID]bool{start.UID: true}

	cur := start
	for {
		refs := cur.Object.GetOwnerReferences()
		if len(refs) == 0 {
			break
		}
		ref := pickController(refs)
		gvr, err := o.fetchAncestorGVR(ref)
		if err != nil {
			break // Kind not mappable — stop rather than fail the render.
		}
		parentObj, err := o.dynClient.Resource(gvr).Namespace(cur.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to fetch ancestor %s/%s: %w", ref.Kind, ref.Name, err)
		}
		parent := nodeFromUnstructured(parentObj)
		if visited[parent.UID] {
			break // cycle
		}
		visited[parent.UID] = true
		chain = append(chain, parent)
		cur = parent
	}

	// Linear parent→child list: chain[len-1] is topmost ancestor.
	root := chain[len(chain)-1]
	prev := root
	for i := len(chain) - 2; i >= 0; i-- {
		prev.Children = []*TreeNode{chain[i]}
		prev = chain[i]
	}
	RenderTree(root, o.Out, o.MaxDepth)
	return nil
}

func pickController(refs []metav1.OwnerReference) metav1.OwnerReference {
	for _, r := range refs {
		if r.Controller != nil && *r.Controller {
			return r
		}
	}
	return refs[0]
}

func splitAPIVersion(apiVersion string) (group, version string) {
	if i := strings.Index(apiVersion, "/"); i >= 0 {
		return apiVersion[:i], apiVersion[i+1:]
	}
	return "", apiVersion
}

// fetchAncestorGVR resolves an ownerReference's Kind+APIVersion to a GVR.
func (o *TreeOptions) fetchAncestorGVR(ref metav1.OwnerReference) (schema.GroupVersionResource, error) {
	group, version := splitAPIVersion(ref.APIVersion)
	gk := schema.GroupKind{Group: group, Kind: ref.Kind}
	mapping, err := o.restMapper.RESTMapping(gk, version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}
