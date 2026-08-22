package db_test

import (
	"testing"

	"contadinho-go/internal/db"
)

// TestInClauseEmptyYieldsNothingToBuildWith pins the contract every batched
// read path here depends on: `IN ()` is not valid SQL on either driver, so an
// empty id set must produce nothing at all — the signal a caller checks
// before building a query it should not run.
func TestInClauseEmptyYieldsNothingToBuildWith(t *testing.T) {
	in, args := db.InClause(nil)
	if in != "" || args != nil {
		t.Errorf("InClause(nil) = (%q, %v), want (\"\", nil)", in, args)
	}
	if in, args := db.InClause([]string{}); in != "" || args != nil {
		t.Errorf("InClause([]) = (%q, %v), want (\"\", nil)", in, args)
	}
}

// TestInClausePairsBindsWithArgsInOrder pins the halves staying in step —
// the whole reason the two are returned together rather than built
// separately at each call site. Order matters: these are positional binds,
// so an args slice out of step with the placeholders silently queries the
// wrong ids.
func TestInClausePairsBindsWithArgsInOrder(t *testing.T) {
	ids := []string{"a", "b", "c"}
	in, args := db.InClause(ids)
	if in != "?,?,?" {
		t.Errorf("placeholders = %q, want %q", in, "?,?,?")
	}
	if len(args) != len(ids) {
		t.Fatalf("got %d args for %d ids", len(args), len(ids))
	}
	for i, id := range ids {
		if args[i] != any(id) {
			t.Errorf("args[%d] = %v, want %q", i, args[i], id)
		}
	}
}

// TestInClauseSingleID guards the off-by-one the hand-rolled copies this
// replaced were prone to: one id is `?`, with no trailing comma.
func TestInClauseSingleID(t *testing.T) {
	if in, args := db.InClause([]string{"only"}); in != "?" || len(args) != 1 {
		t.Errorf("InClause(one) = (%q, %v), want (\"?\", [only])", in, args)
	}
}
