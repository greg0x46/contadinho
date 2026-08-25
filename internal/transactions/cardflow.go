package transactions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"contadinho-go/internal/dates"
	"contadinho-go/internal/db"
)

// CardDueDates indexes financial_bills so a credit card transaction can be
// projected onto the calendar day its cost actually leaves the paying
// account: the bill's due_date, never its own occurred_at. Two lookup paths
// mirror Pluggy's own transaction metadata: an exact bill (via billId) and,
// failing that, an inferred date from the account's monthly cycle cadence.
type CardDueDates struct {
	byAccount map[string][]cardBill // known bills per account, sorted by due date ascending
	byBillID  map[string]time.Time  // accountID+"\x00"+billID (external or internal) -> due date
}

// cardBill is one known billing cycle: the day it closed (zero when the
// provider hasn't reported one) and the day it is due. The closing day is
// what decides which bill a purchase lands on — a purchase made after a
// bill closed is paid on the *next* cycle, even though that bill's due
// date is still ahead of it on the calendar.
type cardBill struct {
	closing time.Time
	due     time.Time
}

func cardDueKey(accountID, billID string) string {
	return accountID + "\x00" + billID
}

// FetchCardDueDates loads every financial_bills row's due_date (and its
// closing_date when the provider reported one), keyed for both direct
// billId lookup and cadence inference — see ProjectedEntryDate.
func FetchCardDueDates(ctx context.Context, q Querier) (CardDueDates, error) {
	result := CardDueDates{
		byAccount: make(map[string][]cardBill),
		byBillID:  make(map[string]time.Time),
	}
	rows, err := q.QueryContext(ctx, `SELECT account_id, id, external_id, due_date, closing_date FROM financial_bills WHERE due_date IS NOT NULL`)
	if err != nil {
		return CardDueDates{}, fmt.Errorf("query card bill due dates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var accountID, internalID, externalID, dueDateRaw string
		var closingDateRaw sql.NullString
		if err := rows.Scan(&accountID, &internalID, &externalID, &dueDateRaw, &closingDateRaw); err != nil {
			return CardDueDates{}, err
		}
		parsed, err := db.ParseTime(dueDateRaw)
		if err != nil {
			return CardDueDates{}, fmt.Errorf("parse bill due_date %q: %w", dueDateRaw, err)
		}
		bill := cardBill{due: dates.Day(parsed)}
		if closingDateRaw.Valid {
			parsedClosing, err := db.ParseNullTime(closingDateRaw)
			if err != nil {
				return CardDueDates{}, fmt.Errorf("parse bill closing_date %q: %w", closingDateRaw.String, err)
			}
			if parsedClosing != nil {
				bill.closing = dates.Day(*parsedClosing)
			}
		}
		result.byAccount[accountID] = append(result.byAccount[accountID], bill)
		// Pluggy's billId is the external id; the internal id is kept too so
		// locally-created/test data can be matched the same way.
		result.byBillID[cardDueKey(accountID, externalID)] = bill.due
		result.byBillID[cardDueKey(accountID, internalID)] = bill.due
	}
	if err := rows.Err(); err != nil {
		return CardDueDates{}, err
	}
	for accountID := range result.byAccount {
		sort.Slice(result.byAccount[accountID], func(i, j int) bool {
			return result.byAccount[accountID][i].due.Before(result.byAccount[accountID][j].due)
		})
	}
	return result, nil
}

// cardTransactionMetadata is the subset of Pluggy's credit_card_metadata
// this projection reads. billId identifies the closed bill a transaction
// was assigned to; billForecastDate ("YYYY-MM") is Pluggy's own guess at
// which future bill an installment will land on when no bill exists yet.
type cardTransactionMetadata struct {
	BillID           *string `json:"billId"`
	BillForecastDate *string `json:"billForecastDate"`
}

// ProjectedEntryDate returns the calendar day a credit-card transaction's
// cost should be projected onto, in priority order:
//  1. The due_date of the bill named by the transaction's own billId.
//  2. The due_date of the bill matching its billForecastDate month.
//  3. The due date of the first bill that had not closed yet when the
//     purchase happened — projecting the account's own monthly cadence
//     forward past the last known bill — for a forecast month with no
//     bill yet or for a transaction with neither field (still on the
//     open, unbilled cycle).
//
// occurredAt is only the fallback anchor for case 3 — never itself the
// answer, since a card transaction's own date is exactly what this
// projection exists to correct.
func (d CardDueDates) ProjectedEntryDate(accountID string, occurredAt time.Time, cardMetadataRaw *string) time.Time {
	if cardMetadataRaw != nil {
		var meta cardTransactionMetadata
		if err := json.Unmarshal([]byte(*cardMetadataRaw), &meta); err == nil {
			if meta.BillID != nil && *meta.BillID != "" {
				if due, ok := d.byBillID[cardDueKey(accountID, *meta.BillID)]; ok {
					return due
				}
			}
			if meta.BillForecastDate != nil && *meta.BillForecastDate != "" {
				if due, ok := d.dueDateForMonth(accountID, *meta.BillForecastDate); ok {
					return due
				}
			}
		}
	}
	return d.inferredDueDateOnOrAfter(accountID, dates.Day(occurredAt))
}

// dueDateForMonth finds a known due date falling in the given "YYYY-MM"
// month for accountID.
func (d CardDueDates) dueDateForMonth(accountID, yearMonth string) (time.Time, bool) {
	for _, bill := range d.byAccount[accountID] {
		if bill.due.Format("2006-01") == yearMonth {
			return bill.due, true
		}
	}
	return time.Time{}, false
}

// inferredDueDateOnOrAfter returns the due date of the first known bill
// that was still open on `after` — the cycle a purchase made that day
// lands on — or, past the known history, projects the cadence forward
// month by month from the account's last known bill, preserving its
// day-of-month (clamped to shorter months), the same cadence-inference
// approach httpapi's inferredNextClosing uses for bill closing dates.
//
// A bill is "still open" on `after` when it closed on or after that day.
// Comparing against the closing date rather than the due date is what
// keeps a purchase made in the few days between a bill's closing and its
// due date off that already-closed bill: buying on the 9th when the bill
// closed on the 2nd and is due on the 10th is paid on the *next* cycle,
// and dating it to the 10th would project cash leaving on a bill that no
// longer accepts it — a day that is typically already in the past, so the
// cost silently drops out of a forward-looking projection.
//
// Bills the provider reported without a closing date fall back to the due
// date comparison, which is the best available approximation.
func (d CardDueDates) inferredDueDateOnOrAfter(accountID string, after time.Time) time.Time {
	known := d.byAccount[accountID]
	if len(known) == 0 {
		return after
	}
	for _, bill := range known {
		if !bill.closesOn().Before(after) {
			return bill.due
		}
	}
	// last closed before after is guaranteed here: a still-open bill would
	// have been returned by the loop above.
	last := known[len(known)-1]
	dueDay, closingDay := last.due.Day(), last.closing.Day()
	next := last
	for {
		next.due = nextMonthOn(next.due, dueDay)
		if !next.closing.IsZero() {
			next.closing = nextMonthOn(next.closing, closingDay)
		}
		if !next.closesOn().Before(after) {
			return next.due
		}
	}
}

// closesOn is the day this bill stopped accepting purchases: its closing
// date, or its due date when the provider reported no closing.
func (b cardBill) closesOn() time.Time {
	if b.closing.IsZero() {
		return b.due
	}
	return b.closing
}

// nextMonthOn advances date by one month, landing on day-of-month day
// (clamped to the month's actual length).
func nextMonthOn(date time.Time, day int) time.Time {
	nextMonth := time.Date(date.Year(), date.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	if maxDay := dates.DaysInMonth(nextMonth.Year(), nextMonth.Month()); day > maxDay {
		day = maxDay
	}
	return time.Date(nextMonth.Year(), nextMonth.Month(), day, 0, 0, 0, 0, time.UTC)
}

// CreditAccountIDs returns the set of financial_accounts.id whose
// account_type is CREDIT — a credit card's own transactions are debt
// incurred, not cash movement, and must be date-projected via
// ProjectedEntryDate rather than counted on their occurred_at.
func CreditAccountIDs(ctx context.Context, q Querier) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT id FROM financial_accounts WHERE account_type = 'CREDIT'`)
	if err != nil {
		return nil, fmt.Errorf("query credit accounts: %w", err)
	}
	defer rows.Close()
	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// CardMetadataByTransaction batch-loads credit_card_metadata for the given
// transaction ids, for accounts whose transactions need ProjectedEntryDate.
func CardMetadataByTransaction(ctx context.Context, q Querier, transactionIDs []string) (map[string]*string, error) {
	result := make(map[string]*string, len(transactionIDs))
	if len(transactionIDs) == 0 {
		return result, nil
	}
	in, args := db.InClause(transactionIDs)
	query := `SELECT id, credit_card_metadata FROM financial_transactions WHERE id IN (` + in + `)`
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query credit card metadata: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var metadata sql.NullString
		if err := rows.Scan(&id, &metadata); err != nil {
			return nil, err
		}
		if metadata.Valid {
			value := metadata.String
			result[id] = &value
		}
	}
	return result, rows.Err()
}
