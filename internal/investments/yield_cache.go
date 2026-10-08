package investments

import (
	"context"
	"sort"
	"time"
)

// YieldCache keeps what several yield calculations of one request would
// otherwise each load again: an asset's price series. Holdings of the same
// asset (two CDBs with the same terms, a stock bought in two lots) read it
// from the first load. It is meant to live for a single request and is not
// safe for concurrent use. A nil *YieldCache is valid and caches nothing.
type YieldCache struct {
	quotes map[string]cachedQuotes
}

type cachedQuotes struct {
	// until is the last day the series was loaded for; a later one needs a
	// reload, an earlier one is a prefix of what is held.
	until  time.Time
	quotes []AssetQuote
}

func NewYieldCache() *YieldCache {
	return &YieldCache{quotes: map[string]cachedQuotes{}}
}

// assetQuotes is ListAssetQuotes through the cache, always bounded by until.
// The result is shared with other callers and must be treated as read-only.
func (c *YieldCache) assetQuotes(ctx context.Context, q Querier, assetID string, until time.Time) ([]AssetQuote, error) {
	until = Day(until)
	if c == nil {
		return ListAssetQuotes(ctx, q, assetID, &until)
	}
	if held, ok := c.quotes[assetID]; ok && !until.After(held.until) {
		n := sort.Search(len(held.quotes), func(i int) bool { return held.quotes[i].QuotedOn.After(until) })
		return held.quotes[:n], nil
	}
	quotes, err := ListAssetQuotes(ctx, q, assetID, &until)
	if err != nil {
		return nil, err
	}
	c.quotes[assetID] = cachedQuotes{until: until, quotes: quotes}
	return quotes, nil
}
