// Package transactions ports app/transactions/query.py (the filtered,
// paginated, grouped transaction listing with running totals) and the
// manual inclusion/exclusion half of app/transactions/inclusion.py. Category
// assignment lives in package categories; classification, effective money,
// and eligibility rules live in package money.
package transactions

import (
	"context"
	"database/sql"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/money"
)

// Querier is satisfied by both *sql.DB and *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Filters mirrors TransactionFilters. DateFrom/DateTo only take effect as a
// pair — matching the reference, setting just one has no effect.
type Filters struct {
	DateFrom       *money.Date
	DateTo         *money.Date
	Description    *string
	AccountID      *string
	Institution    *string
	CategoryID     *string
	Classification *money.Classification
	ProviderStatus *string
	AmountMin      *decimal.Decimal
	AmountMax      *decimal.Decimal
	Uncategorized  bool
}

// QueryRequest mirrors TransactionQuery.
type QueryRequest struct {
	Timezone string
	GroupBy  money.GroupBy
	Page     int
	PageSize int
	Filters  Filters
}

type AccountSummary struct {
	ID           string
	Name         *string
	Institution  *string
	CurrencyCode *string
}

type EffectiveMoneyView struct {
	Value        string
	CurrencyCode string
	Source       money.Source
}

type Inclusion struct {
	State     money.InclusionState
	ChangedAt *time.Time
	Origin    string
	RuleName  *string
}

type InternalCategory struct {
	ID        string
	Name      string
	Kind      money.CategoryKind
	IsActive  bool
	Icon      string
	Color     string
	Origin    string
	ChangedAt time.Time
}

type TotalsEligibility struct {
	Included bool
	Reason   *money.EligibilityReason
}

// CardInfo is the subset of Pluggy's opaque credit_card_metadata JSON that's
// useful for display: which card a purchase was made on, and — for
// installment purchases — which parcela this row is.
type CardInfo struct {
	Number            string
	InstallmentNumber *int
	TotalInstallments *int
}

// Item mirrors TransactionItem: one row of the query result, with every
// derived field (classification, effective money, eligibility, group key)
// already resolved so the HTTP layer never has to re-run domain logic.
type Item struct {
	ID                      string
	ExternalID              string
	OccurredAt              *time.Time
	Description             *string
	Account                 AccountSummary
	SourceCategory          *string
	InternalCategory        *InternalCategory
	MovementType            *string
	ProviderStatus          *string
	Classification          money.Classification
	Amount                  *string
	CurrencyCode            *string
	AmountInAccountCurrency *string
	EffectiveMoney          *EffectiveMoneyView
	Card                    *CardInfo
	Inclusion               Inclusion
	TotalsEligibility       TotalsEligibility
	GroupKey                string
}

type CurrencyTotals struct {
	CurrencyCode string
	Inflow       string
	Outflow      string
	Balance      string
}

type Group struct {
	Key            string
	Kind           money.PeriodKind
	StartDate      *money.Date
	EndDate        *money.Date
	ItemCount      int
	PageItemCount  int
	HasItemsBefore bool
	HasItemsAfter  bool
	Totals         []CurrencyTotals
}

type Page struct {
	Number     int
	Size       int
	TotalItems int
	TotalPages int
}

type AccountOption struct {
	ID          string
	Name        *string
	Institution *string
}

type CategoryOption struct {
	ID       string
	Name     string
	Kind     money.CategoryKind
	IsActive bool
	Icon     string
	Color    string
}

type AvailableFilters struct {
	Accounts     []AccountOption
	Institutions []string
	Categories   []CategoryOption
}

// Result mirrors TransactionQueryResult.
type Result struct {
	ConfirmedAt      time.Time
	StoredTotal      int
	Page             Page
	Items            []Item
	Totals           []CurrencyTotals
	Groups           []Group
	AvailableFilters AvailableFilters
}
