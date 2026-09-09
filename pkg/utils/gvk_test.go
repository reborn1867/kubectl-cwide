package utils

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestGenerateDirNameByGVK locks in the template directory naming rule:
// lowercased "<Kind>-<Group>-<Version>", using the singular Kind (NOT the
// plural resource). Core-group kinds have an empty group, producing the
// characteristic doubled dash (e.g. pod--v1). These exact names are referenced
// throughout the docs/cookbook and by init/get resolution, so a regression here
// silently breaks template lookup.
func TestGenerateDirNameByGVK(t *testing.T) {
	cases := []struct {
		name string
		gvk  schema.GroupVersionKind
		want string
	}{
		{
			name: "core group Pod -> doubled dash",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			want: "pod--v1",
		},
		{
			name: "core group Service",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			want: "service--v1",
		},
		{
			name: "apps group Deployment",
			gvk:  schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			want: "deployment-apps-v1",
		},
		{
			name: "batch group Job",
			gvk:  schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			want: "job-batch-v1",
		},
		{
			name: "dotted group Ingress",
			gvk:  schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
			want: "ingress-networking.k8s.io-v1",
		},
		{
			name: "autoscaling v2 HPA",
			gvk:  schema.GroupVersionKind{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"},
			want: "horizontalpodautoscaler-autoscaling-v2",
		},
		{
			name: "already-lowercase kind is stable",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "configmap"},
			want: "configmap--v1",
		},
		{
			name: "mixed-case kind is lowercased",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},
			want: "persistentvolumeclaim--v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GenerateDirNameByGVK(tc.gvk); got != tc.want {
				t.Errorf("GenerateDirNameByGVK(%+v) = %q, want %q", tc.gvk, got, tc.want)
			}
		})
	}
}
