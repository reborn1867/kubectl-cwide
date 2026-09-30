package get

import "testing"

func TestNoResourcesMessage(t *testing.T) {
	cases := []struct {
		name string
		o    GetOptions
		want string
	}{
		{
			name: "namespaced single namespace names the namespace",
			o:    GetOptions{Namespace: "kube-system"},
			want: "No resources found in kube-system namespace.",
		},
		{
			name: "all-namespaces must not name a namespace",
			o:    GetOptions{AllNamespaces: true, Namespace: "kube-system"},
			want: "No resources found.",
		},
		{
			name: "empty namespace does not name a namespace",
			o:    GetOptions{Namespace: ""},
			want: "No resources found.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// factory is nil here; anyClusterScoped tolerates that (len(args)==0
			// returns false), so the message reflects only the namespace/-A flags.
			if got := tc.o.noResourcesMessage(); got != tc.want {
				t.Errorf("noResourcesMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}
