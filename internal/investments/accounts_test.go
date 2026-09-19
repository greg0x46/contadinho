package investments_test

import (
	"errors"
	"testing"

	"contadinho-go/internal/investments"
)

func boolRef(v bool) *bool { return &v }

func TestManualAccountActivityIsPersistedByUpdateAccount(t *testing.T) {
	f := newLedgerFixture(t)

	// The edit form saves the flag alongside the name; both must land in the
	// same row, and a later save that omits the flag must not flip it back.
	updated, err := investments.UpdateAccount(f.ctx, f.conn, f.custodyID, investments.AccountInput{
		Name: "Corretora antiga", Active: boolRef(false),
	})
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if updated.Active || updated.Name != "Corretora antiga" {
		t.Fatalf("deactivated account = %+v, want active=false and the new name", updated)
	}
	reloaded, err := investments.GetAccount(f.ctx, f.conn, f.custodyID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if reloaded.Active {
		t.Fatalf("active flag did not persist: %+v", reloaded)
	}

	kept, err := investments.UpdateAccount(f.ctx, f.conn, f.custodyID, investments.AccountInput{Name: "Corretora"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if kept.Active || kept.Name != "Corretora" {
		t.Fatalf("rename without Active = %+v, want the flag untouched", kept)
	}

	reactivated, err := investments.UpdateAccount(f.ctx, f.conn, f.custodyID, investments.AccountInput{
		Name: "Corretora", Active: boolRef(true),
	})
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if !reactivated.Active {
		t.Fatalf("reactivated account = %+v", reactivated)
	}
}

func TestIntegratedAccountCannotBeSwitchedOff(t *testing.T) {
	f := newLedgerFixture(t)
	f.addInvestmentMovement()
	integratedID := "integrated:" + f.sourceID
	before, err := investments.GetAccount(f.ctx, f.conn, integratedID)
	if err != nil {
		t.Fatalf("GetAccount(integrated): %v", err)
	}
	if !before.Active {
		t.Fatalf("grouping starts inactive: %+v", before)
	}

	// The refusal has to come before the write: a link sent in the same
	// request must not be applied while the activity change is rejected.
	_, err = investments.UpdateAccount(f.ctx, f.conn, integratedID, investments.AccountInput{
		FinancialAccountID: &f.bankID, Active: boolRef(false),
	})
	if !errors.Is(err, investments.ErrIntegratedReadOnly) {
		t.Fatalf("deactivate integrated = %v, want ErrIntegratedReadOnly", err)
	}
	after, err := investments.GetAccount(f.ctx, f.conn, integratedID)
	if err != nil {
		t.Fatalf("GetAccount(integrated): %v", err)
	}
	if !after.Active || after.FinancialAccountID != nil || after.Name != before.Name {
		t.Fatalf("refused update changed the row: before=%+v after=%+v", before, after)
	}

	// Restating the current flag is not a change and still lets the link
	// through, which is what the edit form does when it echoes the account.
	linked, err := investments.UpdateAccount(f.ctx, f.conn, integratedID, investments.AccountInput{
		FinancialAccountID: &f.bankID, Active: boolRef(true),
	})
	if err != nil {
		t.Fatalf("link integrated with unchanged Active: %v", err)
	}
	if linked.FinancialAccountID == nil || *linked.FinancialAccountID != f.bankID || !linked.Active {
		t.Fatalf("linked grouping = %+v", linked)
	}
}

// TestFinancialAccountBacksOneCustodyOnly: a bank account linked to a second
// custody would count its cash twice, so the second link is a conflict rather
// than a UNIQUE violation surfacing as a storage error.
func TestFinancialAccountBacksOneCustodyOnly(t *testing.T) {
	f := newLedgerFixture(t)
	linked, err := investments.UpdateAccount(f.ctx, f.conn, f.custodyID, investments.AccountInput{
		Name: "Corretora", FinancialAccountID: &f.bankID,
	})
	if err != nil || linked.FinancialAccountID == nil || *linked.FinancialAccountID != f.bankID {
		t.Fatalf("link first custody = %+v err=%v", linked, err)
	}
	if _, err := investments.CreateAccount(f.ctx, f.conn, investments.AccountInput{
		Name: "Segunda custódia", FinancialAccountID: &f.bankID,
	}); !errors.Is(err, investments.ErrFinancialAccountLinked) {
		t.Fatalf("create with a taken bank account = %v, want ErrFinancialAccountLinked", err)
	}
	other, err := investments.CreateAccount(f.ctx, f.conn, investments.AccountInput{Name: "Segunda custódia"})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := investments.UpdateAccount(f.ctx, f.conn, other.ID, investments.AccountInput{
		Name: "Segunda custódia", FinancialAccountID: &f.bankID,
	}); !errors.Is(err, investments.ErrFinancialAccountLinked) {
		t.Fatalf("update with a taken bank account = %v, want ErrFinancialAccountLinked", err)
	}
	// Restating the account's own link is not a second custody.
	if _, err := investments.UpdateAccount(f.ctx, f.conn, f.custodyID, investments.AccountInput{
		Name: "Corretora renomeada", FinancialAccountID: &f.bankID,
	}); err != nil {
		t.Fatalf("restate own link: %v", err)
	}
}
