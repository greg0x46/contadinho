package investments_test

import (
	"context"
	"errors"
	"testing"

	"contadinho-go/internal/investments"
)

func assetStrPtr(v string) *string { return &v }

// TestCreateAssetRequiresQuoteSourceAndSymbolTogether covers
// normalizeAssetInput's "both or neither" rule: a quote_source with no
// quote_symbol (or vice versa) cannot identify an instrument,
// so CreateAsset must refuse it rather than store it half-configured.
func TestCreateAssetRequiresQuoteSourceAndSymbolTogether(t *testing.T) {
	f := newBookFixture(t)
	ctx := context.Background()

	if _, err := investments.CreateAsset(ctx, f.conn, investments.AssetInput{
		Name: "Bitcoin", AssetType: "Criptoativo", CurrencyCode: "BRL",
		QuoteSource: assetStrPtr("crypto"),
	}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Fatalf("quote_source without quote_symbol: err = %v, want ErrInvalidInput", err)
	}

	if _, err := investments.CreateAsset(ctx, f.conn, investments.AssetInput{
		Name: "Bitcoin", AssetType: "Criptoativo", CurrencyCode: "BRL",
		QuoteSymbol: assetStrPtr("BTC"),
	}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Fatalf("quote_symbol without quote_source: err = %v, want ErrInvalidInput", err)
	}

	asset, err := investments.CreateAsset(ctx, f.conn, investments.AssetInput{
		Name: "Bitcoin", AssetType: "Criptoativo", CurrencyCode: "BRL",
		QuoteSource: assetStrPtr("crypto"), QuoteSymbol: assetStrPtr("BTC"),
	})
	if err != nil {
		t.Fatalf("CreateAsset with both fields set: %v", err)
	}
	if asset.QuoteSource == nil || *asset.QuoteSource != "crypto" ||
		asset.QuoteSymbol == nil || *asset.QuoteSymbol != "BTC" {
		t.Fatalf("created asset = %+v", asset)
	}

	reloaded, err := investments.GetAsset(ctx, f.conn, asset.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if reloaded.QuoteSource == nil || *reloaded.QuoteSource != "crypto" ||
		reloaded.QuoteSymbol == nil || *reloaded.QuoteSymbol != "BTC" {
		t.Fatalf("reloaded asset = %+v", reloaded)
	}

	listed, err := investments.ListAssets(ctx, f.conn)
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(listed) != 1 || listed[0].QuoteSource == nil || *listed[0].QuoteSource != "crypto" {
		t.Fatalf("listed assets = %+v", listed)
	}

	noQuote, err := investments.CreateAsset(ctx, f.conn, investments.AssetInput{
		Name: "Tesouro Selic", AssetType: "Renda fixa", CurrencyCode: "BRL",
	})
	if err != nil {
		t.Fatalf("CreateAsset without quote fields: %v", err)
	}
	if noQuote.QuoteSource != nil || noQuote.QuoteSymbol != nil {
		t.Fatalf("asset without quote fields = %+v", noQuote)
	}
}

// TestUpdateAssetRequiresQuoteSourceAndSymbolTogether mirrors the create
// case for UpdateAsset, and also covers clearing both fields back to nil.
func TestUpdateAssetRequiresQuoteSourceAndSymbolTogether(t *testing.T) {
	f := newBookFixture(t)
	ctx := context.Background()
	asset, err := investments.CreateAsset(ctx, f.conn, investments.AssetInput{
		Name: "Ação X", AssetType: "Ação", CurrencyCode: "BRL",
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if _, err := investments.UpdateAsset(ctx, f.conn, asset.ID, investments.AssetInput{
		Name: "Ação X", AssetType: "Ação", CurrencyCode: "BRL", QuoteSource: assetStrPtr("b3"),
	}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Fatalf("update with only quote_source: err = %v, want ErrInvalidInput", err)
	}

	updated, err := investments.UpdateAsset(ctx, f.conn, asset.ID, investments.AssetInput{
		Name: "Ação X", AssetType: "Ação", CurrencyCode: "BRL",
		QuoteSource: assetStrPtr("b3"), QuoteSymbol: assetStrPtr("PETR4"),
	})
	if err != nil {
		t.Fatalf("UpdateAsset with both fields set: %v", err)
	}
	if updated.QuoteSource == nil || *updated.QuoteSource != "b3" ||
		updated.QuoteSymbol == nil || *updated.QuoteSymbol != "PETR4" {
		t.Fatalf("updated asset = %+v", updated)
	}

	cleared, err := investments.UpdateAsset(ctx, f.conn, asset.ID, investments.AssetInput{
		Name: "Ação X", AssetType: "Ação", CurrencyCode: "BRL",
	})
	if err != nil {
		t.Fatalf("UpdateAsset clearing quote fields: %v", err)
	}
	if cleared.QuoteSource != nil || cleared.QuoteSymbol != nil {
		t.Fatalf("cleared asset = %+v", cleared)
	}
}
