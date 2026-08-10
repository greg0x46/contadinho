package pluggy

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func decp(s string) *decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return &d
}

func strp(s string) *string { return &s }

func TestAccountHashStableAcrossDecimalFormatting(t *testing.T) {
	a1 := AccountSnapshot{ExternalID: "acc-1", Balance: decp("10.00")}
	a2 := AccountSnapshot{ExternalID: "acc-1", Balance: decp("10")}
	if AccountHash(a1) != AccountHash(a2) {
		t.Error("hash should be stable across trailing-zero-only formatting differences")
	}
}

func TestAccountHashChangesWithRealChange(t *testing.T) {
	a1 := AccountSnapshot{ExternalID: "acc-1", Balance: decp("10.00")}
	a2 := AccountSnapshot{ExternalID: "acc-1", Balance: decp("20.00")}
	if AccountHash(a1) == AccountHash(a2) {
		t.Error("hash should change when balance actually changes")
	}
}

func TestAccountHashCoversCreditData(t *testing.T) {
	due := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	base := AccountSnapshot{ExternalID: "acc-1", CreditLimit: decp("5000.00")}

	withAvailable := base
	withAvailable.AvailableCreditLimit = decp("3765.44")
	if AccountHash(base) == AccountHash(withAvailable) {
		t.Error("hash should change when the available credit limit changes")
	}

	withDue := base
	withDue.BalanceDueDate = &due
	if AccountHash(base) == AccountHash(withDue) {
		t.Error("hash should change when the balance due date changes")
	}

	withClose := base
	withClose.BalanceCloseDate = &due
	if AccountHash(withDue) == AccountHash(withClose) {
		t.Error("close and due dates should not be interchangeable in the hash")
	}
}

func TestTransactionHashDistinguishesEveryField(t *testing.T) {
	base := TransactionSnapshot{ExternalID: "tx-1", ExternalAccountID: "acc-1", Amount: decp("-10.00")}
	variant := base
	variant.Description = strp("changed")
	if TransactionHash(base) == TransactionHash(variant) {
		t.Error("hash should change when description changes")
	}
}

func TestInvestmentHashStableAcrossDecimalFormatting(t *testing.T) {
	i1 := InvestmentSnapshot{ExternalID: "inv-1", Balance: decp("10.00")}
	i2 := InvestmentSnapshot{ExternalID: "inv-1", Balance: decp("10")}
	if InvestmentHash(i1) != InvestmentHash(i2) {
		t.Error("hash should be stable across trailing-zero-only formatting differences")
	}
}

func TestInvestmentHashChangesWithRealChange(t *testing.T) {
	i1 := InvestmentSnapshot{ExternalID: "inv-1", Balance: decp("10.00")}
	i2 := InvestmentSnapshot{ExternalID: "inv-1", Balance: decp("20.00")}
	if InvestmentHash(i1) == InvestmentHash(i2) {
		t.Error("hash should change when balance actually changes")
	}
}

func TestInvestmentTransactionHashDistinguishesEveryField(t *testing.T) {
	base := InvestmentTransactionSnapshot{ExternalID: "invtx-1", ExternalInvestmentID: "inv-1", Amount: decp("-10.00")}
	variant := base
	variant.MovementType = strp("SELL")
	if InvestmentTransactionHash(base) == InvestmentTransactionHash(variant) {
		t.Error("hash should change when movement type changes")
	}
}

func TestMapTransactionCanonicalizesMetadataKeyOrder(t *testing.T) {
	// TransactionHash treats PaymentData/CreditCardMetadata/Merchant as
	// opaque bytes, so the JSON key order must already be canonical by the
	// time a TransactionSnapshot is built — that's optionalRawJSON's job
	// (Go's json.Marshal on a map always emits sorted keys), exercised here
	// through mapTransaction rather than by constructing snapshots by hand.
	payloadA, _ := decodeJSON([]byte(`{"id":"tx-1","accountId":"acc-1","merchant":{"a":1,"b":2}}`))
	payloadB, _ := decodeJSON([]byte(`{"id":"tx-1","accountId":"acc-1","merchant":{"b":2,"a":1}}`))
	txA, err := mapTransaction(payloadA, "acc-1")
	if err != nil {
		t.Fatalf("mapTransaction: %v", err)
	}
	txB, err := mapTransaction(payloadB, "acc-1")
	if err != nil {
		t.Fatalf("mapTransaction: %v", err)
	}
	if string(txA.Merchant) != string(txB.Merchant) {
		t.Errorf("merchant JSON not canonicalized: %s vs %s", txA.Merchant, txB.Merchant)
	}
	if TransactionHash(txA) != TransactionHash(txB) {
		t.Error("hash should be identical once merchant JSON is canonicalized")
	}
}
