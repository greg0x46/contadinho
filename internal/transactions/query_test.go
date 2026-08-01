package transactions_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/money"
	"contadinho-go/internal/transactions"
)

func TestQueryEligibilityAndTotals(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{Name: strp("Conta Corrente"), CurrencyCode: strp("BRL")})

	occurred := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	outflowID := f.addTransaction(txn{
		AccountID: acc, Description: strp("Mercado XYZ"), Amount: strp("-50.00"),
		AmountInAccountCurrency: strp("-50.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT"),
	})
	f.setCategory(outflowID, categorySupermercado)

	inflowID := f.addTransaction(txn{
		AccountID: acc, Description: strp("Salario"), Amount: strp("1000.00"),
		AmountInAccountCurrency: strp("1000.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("CREDIT"),
	})
	f.setCategory(inflowID, categorySalario)

	// Uncategorized: categories only label a transaction, they don't gate
	// totals eligibility, so this still counts toward the outflow total.
	uncategorizedID := f.addTransaction(txn{
		AccountID: acc, Description: strp("Sem categoria"), Amount: strp("-5.00"),
		AmountInAccountCurrency: strp("-5.00"), CurrencyCode: strp("BRL"),
		OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT"),
	})

	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupNone, Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.StoredTotal != 3 {
		t.Errorf("StoredTotal = %d, want 3", result.StoredTotal)
	}
	if len(result.Items) != 3 {
		t.Fatalf("len(Items) = %d, want 3", len(result.Items))
	}
	if len(result.Totals) != 1 {
		t.Fatalf("len(Totals) = %d, want 1", len(result.Totals))
	}
	totals := result.Totals[0]
	if totals.CurrencyCode != "BRL" || totals.Inflow != "1000.00" || totals.Outflow != "55.00" || totals.Balance != "945.00" {
		t.Errorf("Totals = %+v", totals)
	}

	for _, item := range result.Items {
		switch item.ID {
		case outflowID, uncategorizedID:
			if !item.TotalsEligibility.Included || item.Classification != money.Outflow {
				t.Errorf("outflow item %s: included=%v classification=%v", item.ID, item.TotalsEligibility.Included, item.Classification)
			}
		case inflowID:
			if !item.TotalsEligibility.Included || item.Classification != money.Inflow {
				t.Errorf("inflow item: included=%v classification=%v", item.TotalsEligibility.Included, item.Classification)
			}
		}
	}
}

func TestQueryFilterByAccount(t *testing.T) {
	f := newFixture(t)
	accA := f.addAccount(account{CurrencyCode: strp("BRL")})
	accB := f.addAccount(account{CurrencyCode: strp("BRL")})
	occurred := time.Now().UTC()

	idA := f.addTransaction(txn{AccountID: accA, Amount: strp("-10.00"), AmountInAccountCurrency: strp("-10.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})
	f.addTransaction(txn{AccountID: accB, Amount: strp("-20.00"), AmountInAccountCurrency: strp("-20.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})

	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupNone, Page: 1, PageSize: 50,
		Filters: transactions.Filters{AccountID: &accA},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != idA {
		t.Errorf("expected only transaction %s, got %+v", idA, result.Items)
	}
}

func TestSpendingByCategory(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	occurred := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	outsideRange := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)

	market1 := f.addTransaction(txn{AccountID: acc, Amount: strp("-30.00"), AmountInAccountCurrency: strp("-30.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})
	f.setCategory(market1, categorySupermercado)
	market2 := f.addTransaction(txn{AccountID: acc, Amount: strp("-20.00"), AmountInAccountCurrency: strp("-20.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})
	f.setCategory(market2, categorySupermercado)

	f.addTransaction(txn{AccountID: acc, Amount: strp("-15.00"), AmountInAccountCurrency: strp("-15.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})

	// Inflow must not count toward spending, even if categorized.
	salary := f.addTransaction(txn{AccountID: acc, Amount: strp("1000.00"), AmountInAccountCurrency: strp("1000.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("CREDIT")})
	f.setCategory(salary, categorySalario)

	// Out of the requested date range: must not count.
	beforeRange := f.addTransaction(txn{AccountID: acc, Amount: strp("-999.00"), AmountInAccountCurrency: strp("-999.00"), CurrencyCode: strp("BRL"), OccurredAt: &outsideRange, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})
	f.setCategory(beforeRange, categorySupermercado)

	from := money.Date{Year: 2026, Month: 3, Day: 1}
	to := money.Date{Year: 2026, Month: 3, Day: 31}
	items, err := transactions.SpendingByCategory(context.Background(), f.conn, transactions.Filters{
		DateFrom: &from, DateTo: &to,
	}, "UTC")
	if err != nil {
		t.Fatalf("SpendingByCategory: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2: %+v", len(items), items)
	}
	if items[0].CategoryID == nil || *items[0].CategoryID != categorySupermercado || items[0].Amount != "50.00" {
		t.Errorf("items[0] = %+v, want category %s amount 50.00", items[0], categorySupermercado)
	}
	if items[1].CategoryID != nil || items[1].CategoryName != "Sem categoria" || items[1].Amount != "15.00" {
		t.Errorf("items[1] = %+v, want uncategorized amount 15.00", items[1])
	}
}

func TestQueryFilterUncategorized(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	occurred := time.Now().UTC()

	categorized := f.addTransaction(txn{AccountID: acc, Amount: strp("-1.00"), AmountInAccountCurrency: strp("-1.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})
	f.setCategory(categorized, categorySupermercado)
	uncategorized := f.addTransaction(txn{AccountID: acc, Amount: strp("-2.00"), AmountInAccountCurrency: strp("-2.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})

	uncategorizedFilter := true
	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupNone, Page: 1, PageSize: 50,
		Filters: transactions.Filters{Uncategorized: uncategorizedFilter},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != uncategorized {
		t.Errorf("expected only uncategorized transaction %s, got %+v", uncategorized, result.Items)
	}
}

func TestQueryAmountRangeFilter(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})
	occurred := time.Now().UTC()

	small := f.addTransaction(txn{AccountID: acc, Amount: strp("-5.00"), AmountInAccountCurrency: strp("-5.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})
	f.addTransaction(txn{AccountID: acc, Amount: strp("-500.00"), AmountInAccountCurrency: strp("-500.00"), CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT")})

	max := mustDecimal(t, "100.00")
	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupNone, Page: 1, PageSize: 50,
		Filters: transactions.Filters{AmountMax: max},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != small {
		t.Errorf("expected only the %s transaction, got %+v", small, result.Items)
	}
}

func TestQueryPaginationOrdersNewestFirst(t *testing.T) {
	f := newFixture(t)
	acc := f.addAccount(account{CurrencyCode: strp("BRL")})

	var ids []string
	for i := 0; i < 5; i++ {
		occurred := time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC)
		id := f.addTransaction(txn{
			AccountID: acc, Amount: strp("-1.00"), AmountInAccountCurrency: strp("-1.00"),
			CurrencyCode: strp("BRL"), OccurredAt: &occurred, ProviderStatus: strp("POSTED"), MovementType: strp("DEBIT"),
		})
		ids = append(ids, id)
	}

	result, err := transactions.Query(context.Background(), f.conn, transactions.QueryRequest{
		Timezone: "UTC", GroupBy: money.GroupNone, Page: 1, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Page.TotalItems != 5 || result.Page.TotalPages != 3 {
		t.Errorf("Page = %+v, want TotalItems=5 TotalPages=3", result.Page)
	}
	if len(result.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(result.Items))
	}
	// Newest occurred_at (ids[4]) must come first.
	if result.Items[0].ID != ids[4] || result.Items[1].ID != ids[3] {
		t.Errorf("Items = [%s, %s], want [%s, %s]", result.Items[0].ID, result.Items[1].ID, ids[4], ids[3])
	}
}

func mustDecimal(t *testing.T, s string) *decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return &d
}
