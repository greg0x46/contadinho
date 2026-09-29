package investments_test

import (
	"errors"
	"reflect"
	"testing"

	"contadinho-go/internal/investments"
)

func redemptionPosition(f *ledgerFixture) string {
	f.t.Helper()
	id := f.addPosition(f.custodyID, "CDB")
	f.create(investments.OperationInput{AccountID: f.custodyID, PositionID: &id,
		Kind: investments.OperationInitialBalance, OccurredOn: day(1), Amount: dec("2000"), Quantity: decRef("3")})
	f.create(investments.OperationInput{AccountID: f.custodyID, PositionID: &id,
		Kind: investments.OperationValuation, OccurredOn: day(2), Amount: dec("2200")})
	return id
}

func redemptionInput(f *ledgerFixture, positionID *string) investments.OperationInput {
	return investments.OperationInput{AccountID: f.custodyID, PositionID: positionID,
		Kind: investments.OperationRedemption, OccurredOn: day(3),
		PrincipalAmount: dec("1000"), IncomeAmount: dec("100"), Fees: dec("5"), Taxes: dec("20")}
}

func assertRedemptionPosition(t *testing.T, f *ledgerFixture, id, cost, quantity, value string) {
	t.Helper()
	p, err := investments.GetPosition(f.ctx, f.conn, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.TotalCost == nil || !p.TotalCost.Equal(dec(cost)) || !p.Quantity.Equal(dec(quantity)) || !p.CurrentValue.Equal(dec(value)) {
		t.Fatalf("position = %+v; want cost=%s quantity=%s value=%s", p, cost, quantity, value)
	}
	f.assertCash("direct redemption", "0")
}

func TestRedemptionPartialFullCorrectionAndDeletion(t *testing.T) {
	f := newLedgerFixture(t)
	id := redemptionPosition(f)
	in := redemptionInput(f, &id)
	in.Amount = dec("99999") // Never trust a client-supplied net amount.
	op := f.create(in)
	if !op.Amount.Equal(dec("1075")) || !op.PrincipalAmount.Equal(dec("1000")) || !op.IncomeAmount.Equal(dec("100")) {
		t.Fatalf("decomposition not persisted: %+v", op)
	}
	assertRedemptionPosition(t, f, id, "1000", "1.5", "1100")
	in.PrincipalAmount = dec("500")
	updated, err := investments.UpdateOperation(f.ctx, f.conn, op.ID, in)
	if err != nil || !updated.Amount.Equal(dec("575")) {
		t.Fatalf("update: %+v %v", updated, err)
	}
	assertRedemptionPosition(t, f, id, "1500", "2.25", "1650")

	in.PrincipalAmount = dec("1500")
	in.OccurredOn = day(4)
	last := f.create(in)
	assertRedemptionPosition(t, f, id, "0", "0", "0")
	in.PrincipalAmount = dec("600")
	in.OccurredOn = day(3)
	if _, err := investments.UpdateOperation(f.ctx, f.conn, op.ID, in); !errors.Is(err, investments.ErrNegativePosition) {
		t.Fatalf("retroactive overdraw must rollback: %v", err)
	}
	assertRedemptionPosition(t, f, id, "0", "0", "0")
	if err := investments.DeleteOperation(f.ctx, f.conn, last.ID); err != nil {
		t.Fatal(err)
	}
	if err := investments.DeleteOperation(f.ctx, f.conn, op.ID); err != nil {
		t.Fatal(err)
	}
	assertRedemptionPosition(t, f, id, "2000", "3", "2200")
}

func TestRedemptionRejectsInvalidComponentsAndOverdraw(t *testing.T) {
	f := newLedgerFixture(t)
	id := redemptionPosition(f)
	cases := map[string]func(*investments.OperationInput){
		"missing position": func(in *investments.OperationInput) { in.PositionID = nil },
		"zero principal":   func(in *investments.OperationInput) { in.PrincipalAmount = dec("0") },
		"negative income":  func(in *investments.OperationInput) { in.IncomeAmount = dec("-1") },
		"negative fees":    func(in *investments.OperationInput) { in.Fees = dec("-1") },
		"negative taxes":   func(in *investments.OperationInput) { in.Taxes = dec("-1") },
		"zero net":         func(in *investments.OperationInput) { in.Taxes = dec("1095") },
		"negative net":     func(in *investments.OperationInput) { in.Taxes = dec("2000") },
		"overdraw":         func(in *investments.OperationInput) { in.PrincipalAmount = dec("2001") },
		"quantity":         func(in *investments.OperationInput) { in.Quantity = decRef("1") },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := redemptionInput(f, &id)
			change(&in)
			if _, err := investments.CreateOperation(f.ctx, f.conn, in); err == nil {
				t.Fatal("invalid redemption accepted")
			}
			assertRedemptionPosition(t, f, id, "2000", "3", "2200")
		})
	}
}

func TestRedemptionReconciliationIsAtomicAndRequiresFullNet(t *testing.T) {
	f := newLedgerFixture(t)
	id := redemptionPosition(f)
	in := redemptionInput(f, &id)
	bank := f.addBankTransaction("1075", day(3))
	link := investments.ReconciliationInput{FinancialTransactionID: &bank, Amount: dec("1000")}
	if _, err := investments.CreateOperations(f.ctx, f.conn, []investments.OperationInput{in}, &link); err == nil {
		t.Fatal("partial net accepted")
	}
	assertRedemptionPosition(t, f, id, "2000", "3", "2200")
	ops, err := investments.ListOperations(f.ctx, f.conn, investments.OperationFilter{})
	if err != nil || len(ops) != 2 {
		t.Fatalf("failed create left an orphan: %+v %v", ops, err)
	}
	link.Amount = dec("1075")
	ops, err = investments.CreateOperations(f.ctx, f.conn, []investments.OperationInput{in}, &link)
	if err != nil {
		t.Fatal(err)
	}
	op := ops[0]
	if _, err := investments.UpdateOperation(f.ctx, f.conn, op.ID, in); !errors.Is(err, investments.ErrOperationHasReconciliations) {
		t.Fatalf("linked update = %v", err)
	}
	if err := investments.DeleteOperation(f.ctx, f.conn, op.ID); !errors.Is(err, investments.ErrOperationHasReconciliations) {
		t.Fatalf("linked delete = %v", err)
	}
	links, err := investments.ListReconciliations(f.ctx, f.conn, investments.ReconciliationFilter{OperationID: &op.ID})
	if err != nil || len(links) != 1 {
		t.Fatalf("links: %+v %v", links, err)
	}
	if err := investments.DeleteReconciliation(f.ctx, f.conn, links[0].ID); err != nil {
		t.Fatal(err)
	}
	assertRedemptionPosition(t, f, id, "1000", "1.5", "1100")
	if err := investments.DeleteOperation(f.ctx, f.conn, op.ID); err != nil {
		t.Fatal(err)
	}
	assertRedemptionPosition(t, f, id, "2000", "3", "2200")
}

func TestRedemptionReportingCompositionSurvivesLinkAndUnlink(t *testing.T) {
	for _, tc := range []struct{ income, fees, taxes, net string }{
		{"100", "5", "20", "1075"},
		{"0", "0", "0", "1000"},
		{"5", "10", "20", "975"},
		{"0.03", "0.01", "0.01", "1000.01"},
	} {
		t.Run(tc.net, func(t *testing.T) {
			f := newLedgerFixture(t)
			id := redemptionPosition(f)
			in := redemptionInput(f, &id)
			in.IncomeAmount, in.Fees, in.Taxes = dec(tc.income), dec(tc.fees), dec(tc.taxes)
			op := f.create(in)
			if !op.Amount.Equal(dec(tc.net)) {
				t.Fatalf("net=%s", op.Amount)
			}
			before, err := investments.ManualOperationReporting(f.ctx, f.conn)
			if err != nil {
				t.Fatal(err)
			}
			if !before.Income.Equal(dec(tc.income)) || !before.Fees.Equal(dec(tc.fees)) || !before.Taxes.Equal(dec(tc.taxes)) {
				t.Fatalf("report = %+v", before)
			}
			bank := f.addBankTransaction(tc.net, day(3))
			link := f.link(op.ID, bank, tc.net)
			after, err := investments.ManualOperationReporting(f.ctx, f.conn)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("report after link=%+v %v", after, err)
			}
			excluded, err := investments.ReconciledRedemptionAmounts(f.ctx, f.conn, &bank)
			if err != nil || !excluded[bank].Equal(dec(tc.net)) {
				t.Fatalf("excluded=%+v %v", excluded, err)
			}
			if err := investments.DeleteReconciliation(f.ctx, f.conn, link.ID); err != nil {
				t.Fatal(err)
			}
			after, err = investments.ManualOperationReporting(f.ctx, f.conn)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("report after unlink=%+v %v", after, err)
			}
		})
	}
}

func TestIntegratedRedemptionDoesNotChangeProviderBalances(t *testing.T) {
	f := newLedgerFixture(t)
	f.addInvestmentMovement()
	accounts, err := investments.ListAccounts(f.ctx, f.conn)
	if err != nil {
		t.Fatal(err)
	}
	var accountID string
	for _, account := range accounts {
		if account.Kind == investments.AccountKindIntegrated {
			accountID = account.ID
		}
	}
	if accountID == "" {
		t.Fatal("missing integrated account")
	}
	before, err := investments.ListPositions(f.ctx, f.conn, investments.PositionFilter{AccountID: &accountID})
	if err != nil {
		t.Fatal(err)
	}
	in := redemptionInput(f, nil)
	in.AccountID = accountID
	op := f.create(in)
	bank := f.addBankTransaction("1075", day(3))
	f.link(op.ID, bank, "1075")
	after, err := investments.ListPositions(f.ctx, f.conn, investments.PositionFilter{AccountID: &accountID})
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("provider holdings changed: %+v %v", after, err)
	}
}

func TestMultipleRedemptionsCanExplainOneBankCredit(t *testing.T) {
	f := newLedgerFixture(t)
	id := redemptionPosition(f)
	bank := f.addBankTransaction("2150", day(3))
	in := redemptionInput(f, &id)
	first, second := f.create(in), f.create(in)
	f.link(first.ID, bank, "1075")
	f.link(second.ID, bank, "1075")
	excluded, err := investments.ReconciledRedemptionAmounts(f.ctx, f.conn, &bank)
	if err != nil || !excluded[bank].Equal(dec("2150")) {
		t.Fatalf("excluded=%+v %v", excluded, err)
	}
	assertRedemptionPosition(t, f, id, "0", "0", "0")
}
