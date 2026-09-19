package investments

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

func assetCanonicalKey(name string, ticker *string, assetType string) string {
	clean := func(value string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToUpper(r)
			}
			return -1
		}, value)
	}
	if ticker != nil && clean(*ticker) != "" {
		return "ticker:" + clean(*ticker)
	}
	return "name:" + clean(assetType) + ":" + clean(name)
}

func scanAsset(row interface{ Scan(...any) error }) (Asset, error) {
	var asset Asset
	var ticker sql.NullString
	var created, updated string
	err := row.Scan(&asset.ID, &asset.CanonicalKey, &asset.Name, &ticker, &asset.AssetType, &asset.CurrencyCode, &created, &updated)
	if err != nil {
		return Asset{}, err
	}
	if ticker.Valid {
		value := ticker.String
		asset.Ticker = &value
	}
	asset.CreatedAt, err = parseTimestamp(created)
	if err != nil {
		return Asset{}, err
	}
	asset.UpdatedAt, err = parseTimestamp(updated)
	if err != nil {
		return Asset{}, err
	}
	return asset, nil
}

func ListAssets(ctx context.Context, q Querier) ([]Asset, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, canonical_key, name, ticker, asset_type, currency_code, created_at, updated_at FROM investment_assets ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets := []Asset{}
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func GetAsset(ctx context.Context, q Querier, id string) (Asset, error) {
	asset, err := scanAsset(q.QueryRowContext(ctx, `SELECT id, canonical_key, name, ticker, asset_type, currency_code, created_at, updated_at FROM investment_assets WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrAssetNotFound
	}
	return asset, err
}

func normalizeAssetInput(in AssetInput) (AssetInput, string, error) {
	name := strings.TrimSpace(in.Name)
	assetType := strings.TrimSpace(in.AssetType)
	ticker := trimOptional(in.Ticker)
	currency := strings.ToUpper(strings.TrimSpace(in.CurrencyCode))
	if currency == "" {
		currency = "BRL"
	}
	validCurrency := len(currency) == 3
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			validCurrency = false
		}
	}
	if name == "" || assetType == "" || !validCurrency {
		return AssetInput{}, "", ErrInvalidInput
	}
	normalized := AssetInput{Name: name, Ticker: ticker, AssetType: assetType, CurrencyCode: currency}
	return normalized, assetCanonicalKey(name, ticker, assetType), nil
}

func assetKeyExists(ctx context.Context, q Querier, canonicalKey, exceptID string) (bool, error) {
	var count int
	err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM investment_assets WHERE canonical_key = ? AND id <> ?`,
		canonicalKey, exceptID).Scan(&count)
	return count > 0, err
}

func CreateAsset(ctx context.Context, q Querier, in AssetInput) (Asset, error) {
	normalized, key, err := normalizeAssetInput(in)
	if err != nil {
		return Asset{}, err
	}
	exists, err := assetKeyExists(ctx, q, key, "")
	if err != nil {
		return Asset{}, err
	}
	if exists {
		return Asset{}, ErrAssetAlreadyExists
	}
	now, id := db.FormatTime(time.Now()), uuid.NewString()
	if _, err := q.ExecContext(ctx, `
		INSERT INTO investment_assets
			(id, canonical_key, name, ticker, asset_type, currency_code, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, key, normalized.Name, nullableString(normalized.Ticker), normalized.AssetType,
		normalized.CurrencyCode, now, now); err != nil {
		return Asset{}, err
	}
	return GetAsset(ctx, q, id)
}

func UpdateAsset(ctx context.Context, q Querier, id string, in AssetInput) (Asset, error) {
	if _, err := GetAsset(ctx, q, id); err != nil {
		return Asset{}, err
	}
	normalized, key, err := normalizeAssetInput(in)
	if err != nil {
		return Asset{}, err
	}
	exists, err := assetKeyExists(ctx, q, key, id)
	if err != nil {
		return Asset{}, err
	}
	if exists {
		return Asset{}, ErrAssetAlreadyExists
	}
	if _, err := q.ExecContext(ctx, `
		UPDATE investment_assets
		SET canonical_key = ?, name = ?, ticker = ?, asset_type = ?, currency_code = ?, updated_at = ?
		WHERE id = ?`,
		key, normalized.Name, nullableString(normalized.Ticker), normalized.AssetType,
		normalized.CurrencyCode, db.FormatTime(time.Now()), id); err != nil {
		return Asset{}, err
	}
	return GetAsset(ctx, q, id)
}

func DeleteAsset(ctx context.Context, q Querier, id string) error {
	if _, err := GetAsset(ctx, q, id); err != nil {
		return err
	}
	var positionCount int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM investment_positions WHERE asset_id = ?`, id).Scan(&positionCount); err != nil {
		return err
	}
	if positionCount > 0 {
		return ErrAssetHasPositions
	}
	_, err := q.ExecContext(ctx, `DELETE FROM investment_assets WHERE id = ?`, id)
	return err
}

// resolveAsset reuses the canonical instrument or creates it inside the same
// transaction as its first position. An explicit asset id always wins.
func resolveAsset(ctx context.Context, q Querier, in PositionInput) (Asset, error) {
	if in.AssetID != nil && *in.AssetID != "" {
		return GetAsset(ctx, q, *in.AssetID)
	}
	return ensureAsset(ctx, q, in.Name, in.Ticker, in.AssetType, "BRL")
}

func ensureAsset(ctx context.Context, q Querier, rawName string, rawTicker *string, rawKind, rawCurrency string) (Asset, error) {
	name, kind := strings.TrimSpace(rawName), strings.TrimSpace(rawKind)
	if name == "" || kind == "" {
		return Asset{}, ErrInvalidInput
	}
	ticker := trimOptional(rawTicker)
	key := assetCanonicalKey(name, ticker, kind)
	asset, err := scanAsset(q.QueryRowContext(ctx, `SELECT id, canonical_key, name, ticker, asset_type, currency_code, created_at, updated_at FROM investment_assets WHERE canonical_key = ?`, key))
	if err == nil {
		return asset, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Asset{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(rawCurrency))
	if currency == "" {
		currency = "BRL"
	}
	now, id := db.FormatTime(time.Now()), uuid.NewString()
	_, err = q.ExecContext(ctx, `INSERT INTO investment_assets (id, canonical_key, name, ticker, asset_type, currency_code, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, key, name, nullableString(ticker), kind, currency, now, now)
	if err != nil {
		return Asset{}, err
	}
	return GetAsset(ctx, q, id)
}
