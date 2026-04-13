package list

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFilterResources_NamespacedOnly(t *testing.T) {
	resourceLists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", ShortNames: []string{"po"}},
				{Name: "nodes", Namespaced: false, Kind: "Node", ShortNames: []string{"no"}},
				{Name: "pods/status", Namespaced: true, Kind: "Pod"},
			},
		},
	}

	entries := filterResources(resourceLists, false)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "pods" {
		t.Errorf("expected pods, got %s", entries[0].Name)
	}
	if entries[0].ShortNames != "po" {
		t.Errorf("expected shortname 'po', got %q", entries[0].ShortNames)
	}
}

func TestFilterResources_ClusterScoped(t *testing.T) {
	resourceLists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "nodes", Namespaced: false, Kind: "Node"},
				{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
			},
		},
	}

	entries := filterResources(resourceLists, true)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// entries should be sorted alphabetically
	if entries[0].Name != "namespaces" {
		t.Errorf("expected namespaces first (sorted), got %s", entries[0].Name)
	}
	if entries[1].Name != "nodes" {
		t.Errorf("expected nodes second (sorted), got %s", entries[1].Name)
	}
}

func TestFilterResources_SkipsSubresources(t *testing.T) {
	resourceLists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "pods/log", Namespaced: true, Kind: "Pod"},
				{Name: "pods/status", Namespaced: true, Kind: "Pod"},
			},
		},
	}

	entries := filterResources(resourceLists, false)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (subresources filtered), got %d", len(entries))
	}
	if entries[0].Name != "pods" {
		t.Errorf("expected pods, got %s", entries[0].Name)
	}
}

func TestFilterResources_MultipleGroups(t *testing.T) {
	resourceLists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "services", Namespaced: true, Kind: "Service", ShortNames: []string{"svc"}},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment", ShortNames: []string{"deploy"}},
			},
		},
	}

	entries := filterResources(resourceLists, false)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// sorted: deployments < services
	if entries[0].Name != "deployments" {
		t.Errorf("expected deployments first, got %s", entries[0].Name)
	}
	if entries[0].APIVersion != "apps/v1" {
		t.Errorf("expected apps/v1, got %s", entries[0].APIVersion)
	}
	if entries[1].Name != "services" {
		t.Errorf("expected services second, got %s", entries[1].Name)
	}
	if entries[1].APIVersion != "v1" {
		t.Errorf("expected v1, got %s", entries[1].APIVersion)
	}
}

func TestFilterResources_EmptyInput(t *testing.T) {
	entries := filterResources(nil, false)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for nil input, got %d", len(entries))
	}

	entries = filterResources([]*metav1.APIResourceList{}, true)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for empty input, got %d", len(entries))
	}
}
