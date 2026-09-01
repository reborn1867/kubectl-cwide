package get

import (
	"reflect"
	"testing"
)

func TestSplitResourceTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"pod", []string{"pod"}},
		{"pod,svc,cm", []string{"pod", "svc", "cm"}},
		{"pod, svc , cm ", []string{"pod", "svc", "cm"}},
		// TYPE/NAME must NOT be split on '/'; the caller does that separately.
		{"pod/my-pod", []string{"pod/my-pod"}},
		{"pod/my-pod,svc/my-svc", []string{"pod/my-pod", "svc/my-svc"}},
	}
	for _, tc := range cases {
		got := splitResourceTokens(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitResourceTokens(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}
