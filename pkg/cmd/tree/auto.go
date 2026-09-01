package tree

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
)

// runAutoDiscover walks the cluster BFS from the given root, following owner
// references. Every object whose ownerReferences includes an ancestor already
// in the tree becomes a child of that ancestor. No user-configured rules are
// required — this matches the ergonomic default users expect from
// `kubectl tree <kind>/<name>`.
//
// Discovery, listing, and filtering are done in parallel per resource type,
// bounded by autoDiscoverWorkers to avoid stampeding an apiserver with many
// aggregated groups. Each list is bounded by autoDiscoverTimeout for the
// same reason.
func (o *TreeOptions) runAutoDiscover(ctx context.Context, root *TreeNode) error {
	discoveryClient, err := o.factory.ToDiscoveryClient()
	if err != nil {
		return fmt.Errorf("discovery client: %w", err)
	}

	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			return fmt.Errorf("server discovery: %w", err)
		}
		fmt.Fprintf(o.ErrOut, "Warning: some API groups could not be discovered: %v\n", err)
	}

	targets := listableForOwnerScan(resourceLists, o.AllNamespaces)
	if len(targets) == 0 {
		// No listable resources means nothing to walk; render the root alone.
		RenderTree(root, o.Out, o.MaxDepth)
		return nil
	}

	// Fan out one LIST per GVR. Serialize output collation via sync.Mutex —
	// the hot path is I/O-bound, not CPU-bound, so a mutex is cheaper than
	// channels for a fixed-size producer/consumer.
	nodesByUID := map[types.UID]*TreeNode{root.UID: root}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, autoDiscoverWorkers)

	// Collect candidates first (all objects with any ownerRef), so we can
	// stitch them in dependency order regardless of API-group ordering.
	type candidate struct {
		gvk    schema.GroupVersionKind
		obj    *unstructured.Unstructured
		owners []metav1.OwnerReference
	}
	var candidates []candidate

	for _, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(t listableGVR) {
			defer wg.Done()
			defer func() { <-sem }()

			listCtx, cancel := context.WithTimeout(ctx, autoDiscoverTimeout)
			defer cancel()

			var list *unstructured.UnstructuredList
			var err error
			if t.namespaced && !o.AllNamespaces && o.Namespace != "" {
				list, err = o.dynClient.Resource(t.gvr).
					Namespace(o.Namespace).
					List(listCtx, metav1.ListOptions{})
			} else if t.namespaced {
				list, err = o.dynClient.Resource(t.gvr).
					List(listCtx, metav1.ListOptions{})
			} else {
				list, err = o.dynClient.Resource(t.gvr).
					List(listCtx, metav1.ListOptions{})
			}
			if err != nil {
				// Best-effort: skip resource types we can't LIST (RBAC,
				// aggregated apiservers down, etc.) rather than fail the
				// whole walk.
				return
			}

			for i := range list.Items {
				item := &list.Items[i]
				refs := item.GetOwnerReferences()
				if len(refs) == 0 {
					continue
				}
				gvk := item.GroupVersionKind()
				// Preserve Kind from the list metadata when the item's own
				// TypeMeta wasn't set by the server.
				if gvk.Kind == "" {
					gvk.Kind = t.kind
					gvk.Version = t.gvr.Version
					gvk.Group = t.gvr.Group
				}
				mu.Lock()
				candidates = append(candidates, candidate{
					gvk:    gvk,
					obj:    item,
					owners: refs,
				})
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()

	// Multi-pass stitch: any candidate whose owner is already known becomes a
	// child of that owner. Repeat until no candidate is added in a pass.
	// Cycle-safety comes from the "already-attached" check.
	attached := map[types.UID]bool{root.UID: true}
	for {
		progress := false
		remaining := candidates[:0]
		for _, c := range candidates {
			if attached[c.obj.GetUID()] {
				continue
			}
			var parent *TreeNode
			for _, ref := range c.owners {
				if p, ok := nodesByUID[ref.UID]; ok {
					parent = p
					break
				}
			}
			if parent == nil {
				remaining = append(remaining, c)
				continue
			}
			child := nodeFromUnstructured(c.obj)
			parent.Children = append(parent.Children, child)
			nodesByUID[child.UID] = child
			attached[child.UID] = true
			progress = true
		}
		candidates = remaining
		if !progress {
			break
		}
	}

	RenderTree(root, o.Out, o.MaxDepth)
	return nil
}

const (
	// autoDiscoverWorkers caps concurrent LIST requests while scanning every
	// listable resource type for objects owned by anything in the tree.
	autoDiscoverWorkers = 16
	// autoDiscoverTimeout bounds each per-resource LIST so one slow aggregated
	// apiserver can't stall the whole walk.
	autoDiscoverTimeout = 20 * time.Second
)

// listableGVR is the subset of discovery metadata the auto-walker needs.
type listableGVR struct {
	gvr        schema.GroupVersionResource
	kind       string
	namespaced bool
}

// listableForOwnerScan filters the discovery output to resource types that
// we can LIST and that can carry ownerReferences (which is all namespaced
// resources plus non-namespaced ones when -A is set). Subresources are
// dropped. Duplicate GroupResources at multiple versions collapse to one
// entry per GR (the first seen wins — discovery returns preferred version
// first).
func listableForOwnerScan(resourceLists []*metav1.APIResourceList, allNamespaces bool) []listableGVR {
	seen := map[schema.GroupResource]listableGVR{}
	for _, rl := range resourceLists {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range rl.APIResources {
			if strings.Contains(r.Name, "/") {
				continue
			}
			if !containsVerbForOwnerScan(r.Verbs, "list") {
				continue
			}
			// Cluster-scoped resources rarely carry ownerRefs; include them
			// only under -A where the user has opted into the wider scan.
			if !r.Namespaced && !allNamespaces {
				continue
			}
			gr := schema.GroupResource{Group: gv.Group, Resource: r.Name}
			if _, ok := seen[gr]; ok {
				continue
			}
			seen[gr] = listableGVR{
				gvr:        gv.WithResource(r.Name),
				kind:       r.Kind,
				namespaced: r.Namespaced,
			}
		}
	}
	out := make([]listableGVR, 0, len(seen))
	for _, t := range seen {
		out = append(out, t)
	}
	return out
}

func containsVerbForOwnerScan(verbs metav1.Verbs, want string) bool {
	for _, v := range verbs {
		if v == want {
			return true
		}
	}
	return false
}

