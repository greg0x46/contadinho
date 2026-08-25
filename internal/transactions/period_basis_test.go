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

// queryMonthOf asks for one calendar month the way the /transacoes screen
// does — a date_from/date_to range — so a test can assert which month a
// transaction is *considered part of*, which is the only thing the
// period_basis preference decides. Grouping and ordering key off the
// purchase date under either basis, so GroupKey can no longer answer that.
func queryMonthOf(t *testing.T, f *fixture, year int, month time.Month) []transactions.Item {
	t.Helper()
	last := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1).Day()
	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupMonth, Page: 1, PageSize: 50,
		Filters: transactions.Filters{
			DateFrom: &money.Date{Year: year, Month: month, Day: 1},
			DateTo:   &money.Date{Year: year, Month: month, Day: last},
		},
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
// row), a POSTED credit card transaction counts toward the bill's due month
// instead of the purchase month — while still being listed, and grouped,
// under the month it was actually bought in.
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

	if items := queryMonthOf(t, f, 2026, time.April); len(items) != 1 || items[0].ID != id {
		t.Fatalf("April items = %+v, want the purchase (its bill is due in April)", items)
	}
	if items := queryMonthOf(t, f, 2026, time.March); len(items) != 0 {
		t.Fatalf("March items = %+v, want none (the purchase is paid in April)", items)
	}
	// Listed and grouped by when it was bought, not by when it is paid.
	items := queryMonth(t, f)
	if len(items) != 1 || items[0].GroupKey != "month:2026-03" {
		t.Fatalf("items = %+v, want single item grouped under month:2026-03 (the purchase month)", items)
	}
}

// TestPeriodBasisPaidAtEstimatesWhenBillNotResolvedYet confirms a credit
// card transaction whose bill hasn't closed yet (no matching financial_bills
// row — the common case for recent purchases) falls back to occurred_at plus
// one month rather than being dropped or counted in the purchase month.
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

	if items := queryMonthOf(t, f, 2026, time.April); len(items) != 1 || items[0].ID != id {
		t.Fatalf("April items = %+v, want the purchase (estimated to be paid in April)", items)
	}
	if items := queryMonthOf(t, f, 2026, time.March); len(items) != 0 {
		t.Fatalf("March items = %+v, want none (the purchase is estimated to be paid in April)", items)
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

// TestPeriodBasisPaidAtOrdersByPurchaseDateWithinBill confirms that when
// several purchases land on the same bill — so they share one effective date
// and the bill's due date can no longer separate them — they still come back
// newest purchase first, rather than in whatever order their ids happen to
// fall in.
func TestPeriodBasisPaidAtOrdersByPurchaseDateWithinBill(t *testing.T) {
	f := newFixture(t)
	f.setPreference(settings.KeyTransactionsPeriodBasis, settings.PeriodBasisPaidAt)
	acc := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	f.addBill(bill{AccountID: acc, ExternalID: "bill-1", DueDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)})

	addPurchase := func(day int) string {
		occurred := time.Date(2026, 3, day, 12, 0, 0, 0, time.UTC)
		return f.addTransaction(txn{
			AccountID: acc, Amount: strp("-10.00"), AmountInAccountCurrency: strp("-10.00"),
			CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"),
			MovementType: strp("DEBIT"), CreditCardMetadata: strp(`{"billId":"bill-1"}`),
		})
	}
	// Inserted out of order so a stable sort can't pass by accident.
	middle := addPurchase(10)
	newest := addPurchase(20)
	oldest := addPurchase(1)

	items := queryMonth(t, f)
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	want := []string{newest, middle, oldest}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("items[%d].ID = %s, want %s (purchase date descending)", i, items[i].ID, id)
		}
	}
}

// TestPeriodBasisPaidAtListsByPurchaseDateAcrossAccounts pins the shape of
// the /transacoes list under the paid_at basis, where the two dates pull
// hardest in opposite directions: July card purchases and August checking
// transactions all count toward August, but a reader scanning the list still
// wants one run of dates descending, and a week header that never reappears.
func TestPeriodBasisPaidAtListsByPurchaseDateAcrossAccounts(t *testing.T) {
	f := newFixture(t)
	f.setPreference(settings.KeyTransactionsPeriodBasis, settings.PeriodBasisPaidAt)
	card := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("CREDIT")})
	bank := f.addAccount(account{CurrencyCode: strp("BRL"), AccountType: strp("BANK")})

	add := func(acc string, occurred time.Time) string {
		return f.addTransaction(txn{
			AccountID: acc, Amount: strp("-10.00"), AmountInAccountCurrency: strp("-10.00"),
			CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"),
			MovementType: strp("DEBIT"),
		})
	}
	day := func(month time.Month, d int) time.Time {
		return time.Date(2026, month, d, 12, 0, 0, 0, time.UTC)
	}
	// Card purchases are paid a month later, so these July ones fall in
	// August alongside the checking transactions actually made in August.
	// Inserted out of order so a stable sort can't pass by accident.
	bankAug22 := add(bank, day(time.August, 22))
	cardJul24 := add(card, day(time.July, 24))
	bankAug23 := add(bank, day(time.August, 23))
	cardJul30 := add(card, day(time.July, 30))

	items := queryMonthOf(t, f, 2026, time.August)
	want := []string{bankAug23, bankAug22, cardJul30, cardJul24}
	if len(items) != len(want) {
		t.Fatalf("len(items) = %d, want %d — all four are paid in August", len(items), len(want))
	}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("items[%d].ID = %s, want %s (purchase date descending)", i, items[i].ID, id)
		}
	}
	// Every item groups under the week it was bought, and because the sort
	// key and the bucket key are the same date, each week stays contiguous.
	wantKeys := []string{"week:2026-08-17", "week:2026-08-17", "week:2026-07-27", "week:2026-07-20"}
	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupWeek, Page: 1, PageSize: 50,
		Filters: transactions.Filters{
			DateFrom: &money.Date{Year: 2026, Month: time.August, Day: 1},
			DateTo:   &money.Date{Year: 2026, Month: time.August, Day: 31},
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	for i, key := range wantKeys {
		if result.Items[i].GroupKey != key {
			t.Fatalf("items[%d].GroupKey = %s, want %s", i, result.Items[i].GroupKey, key)
		}
	}
	if len(result.Groups) != 3 {
		t.Fatalf("len(Groups) = %d, want 3 distinct, non-repeating week headers", len(result.Groups))
	}
}
