package pluggy

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// The following canonicalize each field type into a string that is stable
// across the exact syntax used to represent an unchanged value (e.g. Pluggy
// re-sending "10.00" vs "10.0" for the same amount, or key order in a
// metadata object), so accountHash/transactionHash only change when the
// underlying data actually does — mirroring canonical_payload's intent,
// though not its exact byte format (the hash is only ever compared against
// this Go implementation's own prior runs, never against the Python
// reference's).

func strField(s *string) string {
	if s == nil {
		return "\x00"
	}
	return *s
}

// decField normalizes away insignificant trailing zeros (and a "-0" sign),
// the same defense canonical_value's Decimal branch applies, so a provider
// resending the same amount with different trailing-zero formatting doesn't
// register as a change.
func decField(d *decimal.Decimal) string {
	if d == nil {
		return "\x00"
	}
	s := d.String()
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		s = "0"
	}
	return s
}

func timeField(t *time.Time) string {
	if t == nil {
		return "\x00"
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func intField(i *int) string {
	if i == nil {
		return "\x00"
	}
	return strconv.Itoa(*i)
}

func jsonField(raw []byte) string {
	if raw == nil {
		return "\x00"
	}
	return string(raw)
}

func hashFields(kind string, fields ...string) string {
	sum := sha256.Sum256([]byte(kind + "\n" + strings.Join(fields, "\n")))
	return hex.EncodeToString(sum[:])
}

func AccountHash(a AccountSnapshot) string {
	return hashFields("AccountSnapshot",
		a.ExternalID,
		strField(a.Institution),
		strField(a.Name),
		strField(a.Number),
		strField(a.AccountType),
		strField(a.AccountSubtype),
		decField(a.Balance),
		decField(a.CreditLimit),
		strField(a.CurrencyCode),
		timeField(a.ProviderUpdatedAt),
	)
}

func TransactionHash(t TransactionSnapshot) string {
	return hashFields("TransactionSnapshot",
		t.ExternalID,
		t.ExternalAccountID,
		strField(t.Description),
		strField(t.DescriptionRaw),
		decField(t.Amount),
		decField(t.AmountInAccountCurrency),
		decField(t.BalanceAfter),
		strField(t.CurrencyCode),
		timeField(t.OccurredAt),
		strField(t.ProviderStatus),
		strField(t.MovementType),
		strField(t.SourceCategory),
		strField(t.SourceCategoryID),
		strField(t.ProviderCode),
		jsonField(t.PaymentData),
		jsonField(t.CreditCardMetadata),
		jsonField(t.Merchant),
		strField(t.OperationType),
		strField(t.OperationTypeAdditionalInfo),
		strField(t.OperationCategory),
		strField(t.ProviderID),
		intField(t.ProviderOrder),
		timeField(t.ProviderCreatedAt),
		timeField(t.ProviderUpdatedAt),
	)
}
