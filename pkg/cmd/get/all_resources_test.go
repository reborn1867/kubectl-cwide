package get

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSelectListableResources_NamespacedOnly(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"list", "get"}},
				{Name: "pods/status", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get"}},
				{Name: "nodes", Namespaced: false, Kind: "Node", Verbs: metav1.Verbs{"list", "get"}},
				{Name: "bindings", Namespaced: true, Kind: "Binding", Verbs: metav1.Verbs{"create"}},
			},
		},
	}

	got := selectListableResources(lists, false)

	if len(got) != 1 {
		t.Fatalf("expected 1 namespaced listable resource, got %d: %+v", len(got), got)
	}
	if got[0].GVR.Resource != "pods" {
		t.Errorf("expected pods, got %s", got[0].GVR.Resource)
	}
}

func TestSelectListableResources_AllNamespacesIncludesClusterScoped(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"list"}},
				{Name: "nodes", Namespaced: false, Kind: "Node", Verbs: metav1.Verbs{"list"}},
			},
		},
	}

	got := selectListableResources(lists, true)
	if len(got) != 2 {
		t.Fatalf("expected 2 resources with -A, got %d", len(got))
	}
}

func TestSelectListableResources_DedupSameGroupResource(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment", Verbs: metav1.Verbs{"list"}},
			},
		},
		{
			GroupVersion: "apps/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment", Verbs: metav1.Verbs{"list"}},
			},
		},
	}

	got := selectListableResources(lists, false)
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped deployments entry, got %d", len(got))
	}
	if got[0].APIVersion != "apps/v1" {
		t.Errorf("expected apps/v1 kept as preferred, got %s", got[0].APIVersion)
	}
}

func TestSelectListableResources_SkipsSubresourcesAndNonListable(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods/log", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get"}},
				{Name: "componentstatuses", Namespaced: false, Kind: "ComponentStatus", Verbs: metav1.Verbs{"get"}},
			},
		},
	}

	got := selectListableResources(lists, true)
	if len(got) != 0 {
		t.Fatalf("expected 0 listable resources, got %+v", got)
	}
}

func TestTranslateAge(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		created time.Time
		want    string
	}{
		{now.Add(-30 * time.Second), "30s"},
		{now.Add(-5 * time.Minute), "5m"},
		{now.Add(-3 * time.Hour), "3h"},
		{now.Add(-2 * 24 * time.Hour), "2d"},
	}
	for _, c := range cases {
		got := translateAge(metav1.NewTime(c.created), now)
		if got != c.want {
			t.Errorf("translateAge(%v) = %q, want %q", c.created, got, c.want)
		}
	}

	if got := translateAge(metav1.Time{}, now); got != "<unknown>" {
		t.Errorf("zero time: got %q, want <unknown>", got)
	}
}
