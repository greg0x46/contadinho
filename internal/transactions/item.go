package transactions

import (
	"context"
	"fmt"

	"contadinho-go/internal/money"
	"contadinho-go/internal/settings"
)

// GetItem loads one transaction as the very same Item the list paths produce
// — every money rule already applied (classification, effective money,
// totals eligibility), not raw columns.
//
// It exists because callers that need to *decide* something about a single
// transaction (is it eligible to reconcile a recurring commitment?) were
// otherwise forced to re-derive those rules from raw columns, which is how
// two code paths start disagreeing about what a transaction is. The date
// filtering/grouping a QueryRequest carries is meaningless for one row by
// id, so this skips it: buildView still runs so the period-basis preference
// applies identically, but nothing filters the result away.
//
// found is false — rather than an error — when id matches no row, mirroring
// how the store packages report a missing link.
func GetItem(ctx context.Context, q Querier, id string) (item Item, found bool, err error) {
	periodBasis, err := settings.GetTransactionsPeriodBasis(ctx, q)
	if err != nil {
		return Item{}, false, fmt.Errorf("read transactions period basis preference: %w", err)
	}
	billDueDates, err := fetchBillDueDates(ctx, q)
	if err != nil {
		return Item{}, false, err
	}

	rows, err := q.QueryContext(ctx, viewSelect+` WHERE ft.id = ?`, id)
	if err != nil {
		return Item{}, false, fmt.Errorf("query transaction %s: %w", id, err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Item{}, false, err
		}
		return Item{}, false, nil
	}
	r, err := scanRow(rows)
	if err != nil {
		return Item{}, false, err
	}
	if err := rows.Err(); err != nil {
		return Item{}, false, err
	}

	// GroupNone/UTC: buildView only consults the request for grouping, which
	// a single row by id has no use for.
	v, err := buildView(r, QueryRequest{Timezone: "UTC", GroupBy: money.GroupNone}, periodBasis, billDueDates)
	if err != nil {
		return Item{}, false, err
	}
	return toItem(v), true, nil
}
