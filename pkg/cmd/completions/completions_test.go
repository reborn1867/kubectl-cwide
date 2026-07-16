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
