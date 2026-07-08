package marketplace

import "testing"

func TestLockFileUpsertInsertsNew(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(MarketplacePin{Repo: "r", Resource: "pod", Template: "debug", Ref: "v1"})
	if got := len(lf.Pins); got != 1 {
		t.Fatalf("expected 1 pin, got %d", got)
	}
	if lf.Pins[0].Ref != "v1" {
		t.Fatalf("expected ref v1, got %q", lf.Pins[0].Ref)
	}
}

func TestLockFileUpsertReplacesExisting(t *testing.T) {
	lf := &LockFile{Pins: []MarketplacePin{
		{Repo: "r", Resource: "pod", Template: "debug", Ref: "v1"},
	}}
	lf.Upsert(MarketplacePin{Repo: "r", Resource: "pod", Template: "debug", Ref: "v2"})
	if got := len(lf.Pins); got != 1 {
		t.Fatalf("expected 1 pin after replace, got %d", got)
	}
	if lf.Pins[0].Ref != "v2" {
		t.Fatalf("expected ref v2 after replace, got %q", lf.Pins[0].Ref)
	}
}

func TestLockFileUpsertDistinguishesByAllKeys(t *testing.T) {
	lf := &LockFile{Pins: []MarketplacePin{
		{Repo: "r", Resource: "pod", Template: "debug", Ref: "v1"},
	}}
	// Different template — should insert, not replace.
	lf.Upsert(MarketplacePin{Repo: "r", Resource: "pod", Template: "other", Ref: "v2"})
	if got := len(lf.Pins); got != 2 {
		t.Fatalf("expected 2 pins, got %d", got)
	}
}
