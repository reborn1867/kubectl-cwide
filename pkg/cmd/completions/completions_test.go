package completions

import (
	"reflect"
	"testing"
)

func TestFilterPrefix(t *testing.T) {
	all := []string{"pod", "pods", "podsecuritypolicy", "service"}
	cases := []struct {
		prefix string
		want   []string
	}{
		{"", all},
		{"pod", []string{"pod", "pods", "podsecuritypolicy"}},
		{"pods", []string{"pods", "podsecuritypolicy"}},
		{"svc", []string{}},
	}
	for _, tc := range cases {
		got := filterPrefix(all, tc.prefix)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("filterPrefix(%q) = %v; want %v", tc.prefix, got, tc.want)
		}
	}
}

func TestResourceMatchesDir(t *testing.T) {
	cases := []struct {
		dir      string
		resource string
		want     bool
	}{
		// singular Kind, plural resource, singular resource all match the pod dir
		{"pod--v1", "pod", true},
		{"pod--v1", "pods", true},
		// -es plural
		{"ingress-networking.k8s.io-v1", "ingress", true},
		{"ingress-networking.k8s.io-v1", "ingresses", true},
		// -ies plural
		{"networkpolicy-networking.k8s.io-v1", "networkpolicies", true},
		{"networkpolicy-networking.k8s.io-v1", "networkpolicy", true},
		// unpluralized suffix (plural == singular)
		{"endpoints--v1", "endpoints", true},
		// grouped resource
		{"deployment-apps-v1", "deployments", true},
		{"deployment-apps-v1", "deployment", true},
		// non-matches must not over-match
		{"pod--v1", "poddisruptionbudgets", false},
		{"poddisruptionbudget-policy-v1", "pods", false},
		{"deployment-apps-v1", "pods", false},
		{"pod--v1", "svc", false},
		// empty inputs
		{"", "pods", false},
	}
	for _, tc := range cases {
		if got := resourceMatchesDir(tc.dir, tc.resource); got != tc.want {
			t.Errorf("resourceMatchesDir(%q, %q) = %v; want %v", tc.dir, tc.resource, got, tc.want)
		}
	}
}
