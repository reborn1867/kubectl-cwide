package tree

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestListableForOwnerScan_DropsSubresources(t *testing.T) {
	lists := []*metav1.APIResourceList{{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: metav1.Verbs{"list"}},
			{Name: "pods/status", Kind: "Pod", Namespaced: true, Verbs: metav1.Verbs{"get"}},
			{Name: "pods/log", Kind: "Pod", Namespaced: true, Verbs: metav1.Verbs{"get"}},
		},
	}}
	got := listableForOwnerScan(lists, false)
	if len(got) != 1 || got[0].gvr.Resource != "pods" {
		t.Errorf("expected only 'pods' (no subresources); got %+v", got)
	}
}

func TestListableForOwnerScan_ExcludesClusterScopedWithoutAllNamespaces(t *testing.T) {
	lists := []*metav1.APIResourceList{{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "nodes", Kind: "Node", Namespaced: false, Verbs: metav1.Verbs{"list"}},
			{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: metav1.Verbs{"list"}},
		},
	}}
	// Default namespace scope: nodes should be excluded.
	got := listableForOwnerScan(lists, false)
	if len(got) != 1 || got[0].gvr.Resource != "pods" {
		t.Errorf("expected only pods under namespaced scope; got %+v", got)
	}
	// -A: nodes come along too.
	got = listableForOwnerScan(lists, true)
	if len(got) != 2 {
		t.Errorf("expected 2 with -A; got %+v", got)
	}
}

func TestListableForOwnerScan_DropsResourcesWithoutListVerb(t *testing.T) {
	lists := []*metav1.APIResourceList{{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: metav1.Verbs{"list"}},
			// e.g. TokenRequest — has "create" but not "list"
			{Name: "tokenrequests", Kind: "TokenRequest", Namespaced: true, Verbs: metav1.Verbs{"create"}},
		},
	}}
	got := listableForOwnerScan(lists, false)
	if len(got) != 1 || got[0].gvr.Resource != "pods" {
		t.Errorf("only listable pods expected; got %+v", got)
	}
}

func TestListableForOwnerScan_DedupesByGroupResource(t *testing.T) {
	// Same GroupResource served at v1 and v1beta1 (e.g. a Kind during
	// GA transition) — the scanner should keep only one entry per GR.
	lists := []*metav1.APIResourceList{
		{GroupVersion: "batch/v1", APIResources: []metav1.APIResource{
			{Name: "cronjobs", Kind: "CronJob", Namespaced: true, Verbs: metav1.Verbs{"list"}},
		}},
		{GroupVersion: "batch/v1beta1", APIResources: []metav1.APIResource{
			{Name: "cronjobs", Kind: "CronJob", Namespaced: true, Verbs: metav1.Verbs{"list"}},
		}},
	}
	got := listableForOwnerScan(lists, false)
	if len(got) != 1 {
		t.Errorf("expected dedup to 1 entry; got %+v", got)
	}
}
