package investments_test

import (
	"context"
	"errors"
	"testing"

	"contadinho-go/internal/investments"
)

func TestAssetClassificationSeparatesExposureFromInstrumentAndPreservesLegacyTypes(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want investments.AssetClass
	}{
		{"FIXED_INCOME", investments.AssetClassFixedIncome},
		{"AÇÃO", investments.AssetClassVariableIncome},
		{"ETF de renda fixa", investments.AssetClassFixedIncome},
		{"ETF de criptoativos", investments.AssetClassCrypto},
		{"ETF", investments.AssetClassOther},
		{"Fundo", investments.AssetClassOther},
		{"PGBL", investments.AssetClassOther},
	} {
		if got := investments.InferAssetClass(tc.kind); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.kind, got, tc.want)
		}
	}
	f := newBookFixture(t)
	ctx := context.Background()
	asset, err := investments.CreateAsset(ctx, f.conn, investments.AssetInput{
		Name: "Fundo importado", AssetType: "MUTUAL_FUND", AssetClass: investments.AssetClassMultimarket,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := investments.UpdateAsset(ctx, f.conn, asset.ID, investments.AssetInput{Name: "Fundo renomeado", AssetType: "MUTUAL_FUND"})
	if err != nil || updated.AssetClass != investments.AssetClassMultimarket || updated.AssetType != "MUTUAL_FUND" {
		t.Fatalf("legacy update = %+v, err = %v", updated, err)
	}
	for _, in := range []investments.AssetInput{
		{Name: "Classe inválida", AssetType: "CDB", AssetClass: "connection"},
		{Name: "Tipo incompatível", AssetType: "CDB", AssetClass: investments.AssetClassVariableIncome},
	} {
		if _, err := investments.CreateAsset(ctx, f.conn, in); !errors.Is(err, investments.ErrInvalidInput) {
			t.Fatalf("invalid classification: %v", err)
		}
	}
}

func TestAssetSeedIsIdempotentPreservesExistingAssetsAndCreatesNoHoldings(t *testing.T) {
	f := newBookFixture(t)
	ctx := context.Background()
	ticker := "petr4.sa"
	existing, err := investments.CreateAsset(ctx, f.conn, investments.AssetInput{Name: "Minha Petrobras", Ticker: &ticker, AssetType: "Ação"})
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := investments.SeedInvestmentAssets(ctx, f.conn)
	if err != nil || inserted != 26 {
		t.Fatalf("seed = %d, %v, want 26 additions", inserted, err)
	}
	if count, err := investments.SeedInvestmentAssets(ctx, f.conn); err != nil || count != 0 {
		t.Fatalf("repeat = %d, %v", count, err)
	}
	preserved, err := investments.GetAsset(ctx, f.conn, existing.ID)
	if err != nil || preserved.Name != existing.Name || preserved.QuoteSource != nil {
		t.Fatalf("existing was changed: %+v, %v", preserved, err)
	}
	assets, err := investments.ListAssets(ctx, f.conn)
	if err != nil || len(assets) != 27 {
		t.Fatalf("assets = %d, %v", len(assets), err)
	}
	for _, asset := range assets {
		if asset.Ticker == nil {
			continue
		}
		switch *asset.Ticker {
		case "HASH11":
			if asset.AssetClass != investments.AssetClassCrypto || asset.QuoteSource == nil || *asset.QuoteSource != "b3" {
				t.Fatalf("crypto ETF = %+v", asset)
			}
		case "LFTB11":
			if asset.AssetClass != investments.AssetClassFixedIncome || asset.QuoteSource == nil || *asset.QuoteSource != "b3" {
				t.Fatalf("bond ETF = %+v", asset)
			}
		case "BTC":
			if asset.AssetClass != investments.AssetClassCrypto || asset.QuoteSource == nil || *asset.QuoteSource != "crypto" {
				t.Fatalf("Bitcoin = %+v", asset)
			}
			// A renamed symbol changes canonical identity, but the stable seed ID
			// still identifies this entry and must not cause a duplicate-key error.
			renamedTicker := "XBT"
			if _, err := investments.UpdateAsset(ctx, f.conn, asset.ID, investments.AssetInput{
				Name: "Meu Bitcoin", Ticker: &renamedTicker, AssetType: asset.AssetType, AssetClass: asset.AssetClass,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if count, err := investments.SeedInvestmentAssets(ctx, f.conn); err != nil || count != 0 {
		t.Fatalf("seed after editing = %d, %v", count, err)
	}
	assets, _ = investments.ListAssets(ctx, f.conn)
	for _, asset := range assets {
		if asset.Ticker != nil && *asset.Ticker == "XBT" && (asset.Name != "Meu Bitcoin" || asset.QuoteSource != nil) {
			t.Fatalf("user preferences lost: %+v", asset)
		}
	}
	for _, table := range []string{"investment_positions", "investment_accounts", "investment_operations", "investment_asset_quotes"} {
		var count int
		if err := f.conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s = %d, %v, want no financial rows", table, count, err)
		}
	}
}
