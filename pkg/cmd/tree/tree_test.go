package tree

import (
	"testing"

	"github.com/kubectl-cwide/pkg/models"
)

func TestParseRelatedFlag(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		want    models.TreeRelation
		wantErr bool
	}{
		{
			name: "two parts",
			flag: "replicasets:ownerRef",
			want: models.TreeRelation{
				Resource: "replicasets",
				Bind:     models.TreeBindSpec{Type: "ownerRef"},
			},
		},
		{
			name: "three parts with parent",
			flag: "pods:ownerRef:replicasets",
			want: models.TreeRelation{
				Resource: "pods",
				Bind:     models.TreeBindSpec{Type: "ownerRef", Parent: "replicasets"},
			},
		},
		{
			name: "fieldRef",
			flag: "configmaps:fieldRef",
			want: models.TreeRelation{
				Resource: "configmaps",
				Bind:     models.TreeBindSpec{Type: "fieldRef"},
			},
		},
		{
			name:    "invalid single part",
			flag:    "replicasets",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRelatedFlag(tt.flag)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Resource != tt.want.Resource {
				t.Errorf("resource: got %q, want %q", got.Resource, tt.want.Resource)
			}
			if got.Bind.Type != tt.want.Bind.Type {
				t.Errorf("bind type: got %q, want %q", got.Bind.Type, tt.want.Bind.Type)
			}
			if got.Bind.Parent != tt.want.Bind.Parent {
				t.Errorf("bind parent: got %q, want %q", got.Bind.Parent, tt.want.Bind.Parent)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name         string
		arg          string
		wantResource string
		wantName     string
		wantErr      bool
	}{
		{name: "valid", arg: "deployment/nginx", wantResource: "deployment", wantName: "nginx"},
		{name: "valid with dots", arg: "statefulset/my-app.v2", wantResource: "statefulset", wantName: "my-app.v2"},
		{name: "no slash", arg: "deployment-nginx", wantErr: true},
		{name: "empty name", arg: "deployment/", wantErr: true},
		{name: "empty type", arg: "/nginx", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &TreeOptions{}
			parts := splitArg(tt.arg)
			if parts == nil {
				if !tt.wantErr {
					t.Fatal("expected valid parse, got error")
				}
				return
			}
			o.rootResource = parts[0]
			o.rootName = parts[1]

			if tt.wantErr {
				t.Fatal("expected error, got valid parse")
			}
			if o.rootResource != tt.wantResource {
				t.Errorf("resource: got %q, want %q", o.rootResource, tt.wantResource)
			}
			if o.rootName != tt.wantName {
				t.Errorf("name: got %q, want %q", o.rootName, tt.wantName)
			}
		})
	}
}

// splitArg is a test helper that mirrors the arg parsing in Complete.
func splitArg(arg string) []string {
	parts := make([]string, 2)
	idx := 0
	for i, c := range arg {
		if c == '/' {
			parts[0] = arg[:i]
			parts[1] = arg[i+1:]
			idx++
			break
		}
	}
	if idx == 0 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	return parts
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    TreeOptions
		wantErr string
	}{
		{
			name: "valid ownerRef",
			opts: TreeOptions{
				rootResource: "deployment",
				rootName:     "nginx",
				relations: []models.TreeRelation{
					{Resource: "replicasets", Bind: models.TreeBindSpec{Type: "ownerRef"}},
				},
			},
		},
		{
			name: "no relations",
			opts: TreeOptions{
				rootResource: "deployment",
				rootName:     "nginx",
			},
			wantErr: "at least one relation",
		},
		{
			name: "invalid bind type",
			opts: TreeOptions{
				rootResource: "deployment",
				rootName:     "nginx",
				relations: []models.TreeRelation{
					{Resource: "pods", Bind: models.TreeBindSpec{Type: "magic"}},
				},
			},
			wantErr: "invalid bind type",
		},
		{
			name: "fieldRef without path",
			opts: TreeOptions{
				rootResource: "deployment",
				rootName:     "nginx",
				relations: []models.TreeRelation{
					{Resource: "configmaps", Bind: models.TreeBindSpec{Type: "fieldRef"}},
				},
			},
			wantErr: "requires a path",
		},
		{
			name: "fieldRef with path",
			opts: TreeOptions{
				rootResource: "deployment",
				rootName:     "nginx",
				relations: []models.TreeRelation{
					{Resource: "configmaps", Bind: models.TreeBindSpec{Type: "fieldRef", Path: ".spec.volumes[*].configMap.name"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
