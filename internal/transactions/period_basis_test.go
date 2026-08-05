package transactions_test

import (
	"context"
	"testing"
	"time"

	"contadinho-go/internal/money"
	"contadinho-go/internal/settings"
	"contadinho-go/internal/transactions"
)

func queryMonth(t *testing.T, f *fixture) []transactions.Item {
	t.Helper()
	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupMonth, Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return result.Items
}

// TestPeriodBasisDefaultsToOccurredAt confirms the transactions.period_basis
// preference defaults to today's behavior (grouped by purchase date) when
// the user never set it, even for a credit card account.
func TestPeriodBasisDefaultsToOccurredAt(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	occurred := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	id := f.addTransaction(txn{
		AccountID: acc, Amount: strp("-10.00"), AmountInAccountCurrency: strp("-10.00"),
		CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("PENDING"),
		MovementType: strp("DEBIT"), CreditCardMetadata: strp(`{"billId":"bill-1"}`),
	})

	items := queryMonth(t, f)
	if len(items) != 1 || items[0].ID != id || items[0].GroupKey != "month:2026-03" {
		t.Fatalf("items = %+v, want single item grouped under month:2026-03", items)
	}
}

// TestPeriodBasisPaidAtUsesBillDueDate confirms that once a bill has
// resolved (billId in credit_card_metadata matches a synced financial_bills
// row), a POSTED credit card transaction groups by the bill's due month
// instead of the purchase month.
func TestPeriodBasisPaidAtUsesBillDueDate(t *testing.T) {
	f := newFixture(t)
	f.setPreference(settings.KeyTransactionsPeriodBasis, settings.PeriodBasisPaidAt)
	acc := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	f.addBill(bill{AccountID: acc, ExternalID: "bill-1", DueDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)})

	occurred := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	id := f.addTransaction(txn{
		AccountID: acc, Amount: strp("-10.00"), AmountInAccountCurrency: strp("-10.00"),
		CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"),
		MovementType: strp("DEBIT"), CreditCardMetadata: strp(`{"billId":"bill-1"}`),
	})

	items := queryMonth(t, f)
	if len(items) != 1 || items[0].ID != id || items[0].GroupKey != "month:2026-04" {
		t.Fatalf("items = %+v, want single item grouped under month:2026-04 (the bill's due month)", items)
	}
}

// TestPeriodBasisPaidAtEstimatesWhenBillNotResolvedYet confirms a credit
// card transaction whose bill hasn't closed yet (no matching financial_bills
// row — the common case for recent purchases) falls back to occurred_at plus
// one month rather than being dropped or left on the purchase month.
func TestPeriodBasisPaidAtEstimatesWhenBillNotResolvedYet(t *testing.T) {
	f := newFixture(t)
	f.setPreference(settings.KeyTransactionsPeriodBasis, settings.PeriodBasisPaidAt)
	acc := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})

	occurred := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	id := f.addTransaction(txn{
		AccountID: acc, Amount: strp("-10.00"), AmountInAccountCurrency: strp("-10.00"),
		CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("PENDING"),
		MovementType: strp("DEBIT"), CreditCardMetadata: strp(`{"billId":"bill-not-synced-yet"}`),
	})

	items := queryMonth(t, f)
	if len(items) != 1 || items[0].ID != id || items[0].GroupKey != "month:2026-04" {
		t.Fatalf("items = %+v, want single item grouped under month:2026-04 (occurred_at + 1 month estimate)", items)
	}
}

// TestPeriodBasisPaidAtDoesNotAffectNonCreditAccounts confirms the paid_at
// preference only changes credit card transactions — a checking account
// transaction has no purchase/payment lag, so it always groups by
// occurred_at regardless of the preference.
func TestPeriodBasisPaidAtDoesNotAffectNonCreditAccounts(t *testing.T) {
	f := newFixture(t)
	f.setPreference(settings.KeyTransactionsPeriodBasis, settings.PeriodBasisPaidAt)
	acc := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("BANK")})

	occurred := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	id := f.addTransaction(txn{
		AccountID: acc, Amount: strp("-10.00"), AmountInAccountCurrency: strp("-10.00"),
		CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"),
		MovementType: strp("DEBIT"),
	})

	items := queryMonth(t, f)
	if len(items) != 1 || items[0].ID != id || items[0].GroupKey != "month:2026-03" {
		t.Fatalf("items = %+v, want single item grouped under month:2026-03 (occurred_at, unaffected)", items)
	}
}
