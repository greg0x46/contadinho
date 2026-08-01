package pluggy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/shopspring/decimal"
)

// MappingError mirrors MappingError: a problem with a single provider
// payload, always carrying the same (code, safe_message) pair a
// ProviderError needs.
type MappingError struct {
	Code        string
	SafeMessage string
	ExternalID  *string
}

func (e *MappingError) Error() string { return e.SafeMessage }

func mappingErr(field, reason string) *MappingError {
	return &MappingError{Code: "invalid_provider_payload", SafeMessage: fmt.Sprintf("%s %s", field, reason)}
}

// decodeJSON mirrors decode_json: the root must be a JSON object, and every
// number decodes as json.Number (a string under the hood) rather than
// float64, so amounts are never rounded through a float — matching the
// reference's parse_float=Decimal. Go's decoder already rejects non-finite
// literals (NaN/Infinity aren't valid JSON syntax at all), so no equivalent
// of the reference's parse_constant hook is needed.
func decodeJSON(content []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return nil, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Provider returned invalid JSON"}
	}
	obj, ok := decoded.(map[string]any)
	if !ok {
		return nil, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Provider JSON root must be an object"}
	}
	return obj, nil
}

func optionalString(value any, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	s, ok := value.(string)
	if !ok {
		return nil, mappingErr(field, "must be text or null")
	}
	return &s, nil
}

func requiredID(value any, entity string) (string, error) {
	s, ok := value.(string)
	if !ok || s == "" {
		return "", mappingErr(entity, "is missing a usable external identifier")
	}
	return s, nil
}

func optionalDecimal(value any, field string) (*decimal.Decimal, error) {
	if value == nil {
		return nil, nil
	}
	num, ok := value.(json.Number)
	if !ok {
		return nil, mappingErr(field, "must be an exact number")
	}
	d, err := decimal.NewFromString(string(num))
	if err != nil {
		return nil, mappingErr(field, "is invalid")
	}
	return &d, nil
}

// optionalDateTime mirrors _optional_datetime. RFC3339Nano's "Z07:00" token
// requires the input to carry an explicit zone (either "Z" or a numeric
// offset); time.Parse itself rejects a timestamp missing one, so no separate
// check is needed to reject a timezone-less string like the reference does.
func optionalDateTime(value any, field string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	s, ok := value.(string)
	if !ok {
		return nil, mappingErr(field, "must be an ISO timestamp")
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return nil, mappingErr(field, "is invalid")
	}
	utc := t.UTC()
	return &utc, nil
}

func optionalInteger(value any, field string) (*int, error) {
	d, err := optionalDecimal(value, field)
	if err != nil || d == nil {
		return nil, err
	}
	if !d.Equal(d.Truncate(0)) {
		return nil, mappingErr(field, "must be an integer")
	}
	i := int(d.IntPart())
	return &i, nil
}

func optionalObject(value any, field string) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, mappingErr(field, "must be an object or null")
	}
	return obj, nil
}

// optionalRawJSON re-serializes value (already decoded via decodeJSON, so
// numbers are json.Number) as canonical JSON — Go's encoding/json always
// emits map keys sorted, so this is deterministic without extra work.
func optionalRawJSON(value any, field string) ([]byte, error) {
	obj, err := optionalObject(value, field)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, mappingErr(field, "could not be serialized")
	}
	return out, nil
}

func mapSource(payload map[string]any, expectedItemID string) (SourceSnapshot, error) {
	externalID, err := requiredID(payload["id"], "Item")
	if err != nil {
		return SourceSnapshot{}, err
	}
	if externalID != expectedItemID {
		return SourceSnapshot{}, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Provider returned an unexpected Item"}
	}
	var institution *string
	if connector, ok := payload["connector"].(map[string]any); ok {
		if institution, err = optionalString(connector["name"], "connector.name"); err != nil {
			return SourceSnapshot{}, err
		}
	}
	status, err := optionalString(payload["status"], "status")
	if err != nil {
		return SourceSnapshot{}, err
	}
	execution, err := optionalString(payload["executionStatus"], "executionStatus")
	if err != nil {
		return SourceSnapshot{}, err
	}
	updatedAt, err := optionalDateTime(payload["lastUpdatedAt"], "lastUpdatedAt")
	if err != nil {
		return SourceSnapshot{}, err
	}
	return SourceSnapshot{
		ExternalItemID:          externalID,
		InstitutionName:         institution,
		ProviderStatus:          status,
		ProviderExecutionStatus: execution,
		ProviderUpdatedAt:       updatedAt,
	}, nil
}

func mapAccount(payload map[string]any, institution *string) (AccountSnapshot, error) {
	externalID, err := requiredID(payload["id"], "Account")
	if err != nil {
		return AccountSnapshot{}, err
	}
	creditData, err := optionalObject(payload["creditData"], "creditData")
	if err != nil {
		return AccountSnapshot{}, err
	}
	var creditLimit *decimal.Decimal
	if creditData != nil {
		if creditLimit, err = optionalDecimal(creditData["creditLimit"], "creditData.creditLimit"); err != nil {
			return AccountSnapshot{}, err
		}
	}
	name, err := optionalString(payload["name"], "name")
	if err != nil {
		return AccountSnapshot{}, err
	}
	number, err := optionalString(payload["number"], "number")
	if err != nil {
		return AccountSnapshot{}, err
	}
	accountType, err := optionalString(payload["type"], "type")
	if err != nil {
		return AccountSnapshot{}, err
	}
	accountSubtype, err := optionalString(payload["subtype"], "subtype")
	if err != nil {
		return AccountSnapshot{}, err
	}
	balance, err := optionalDecimal(payload["balance"], "balance")
	if err != nil {
		return AccountSnapshot{}, err
	}
	currencyCode, err := optionalString(payload["currencyCode"], "currencyCode")
	if err != nil {
		return AccountSnapshot{}, err
	}
	return AccountSnapshot{
		ExternalID:     externalID,
		Institution:    institution,
		Name:           name,
		Number:         number,
		AccountType:    accountType,
		AccountSubtype: accountSubtype,
		Balance:        balance,
		CreditLimit:    creditLimit,
		CurrencyCode:   currencyCode,
	}, nil
}

func mapTransaction(payload map[string]any, expectedAccountID string) (TransactionSnapshot, error) {
	externalID, err := requiredID(payload["id"], "Transaction")
	if err != nil {
		return TransactionSnapshot{}, err
	}
	accountID, err := requiredID(payload["accountId"], "Transaction account")
	if err != nil {
		return TransactionSnapshot{}, err
	}
	if accountID != expectedAccountID {
		return TransactionSnapshot{}, &MappingError{
			Code: "unsafe_account_association", SafeMessage: "Transaction does not belong to the requested account",
			ExternalID: &externalID,
		}
	}

	t := TransactionSnapshot{ExternalID: externalID, ExternalAccountID: accountID}

	if t.Description, err = optionalString(payload["description"], "description"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.DescriptionRaw, err = optionalString(payload["descriptionRaw"], "descriptionRaw"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.Amount, err = optionalDecimal(payload["amount"], "amount"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.AmountInAccountCurrency, err = optionalDecimal(payload["amountInAccountCurrency"], "amountInAccountCurrency"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.BalanceAfter, err = optionalDecimal(payload["balance"], "balance"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.CurrencyCode, err = optionalString(payload["currencyCode"], "currencyCode"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.OccurredAt, err = optionalDateTime(payload["date"], "date"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.ProviderStatus, err = optionalString(payload["status"], "status"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.MovementType, err = optionalString(payload["type"], "type"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.SourceCategory, err = optionalString(payload["category"], "category"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.SourceCategoryID, err = optionalString(payload["categoryId"], "categoryId"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.ProviderCode, err = optionalString(payload["providerCode"], "providerCode"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.PaymentData, err = optionalRawJSON(payload["paymentData"], "paymentData"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.CreditCardMetadata, err = optionalRawJSON(payload["creditCardMetadata"], "creditCardMetadata"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.Merchant, err = optionalRawJSON(payload["merchant"], "merchant"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.OperationType, err = optionalString(payload["operationType"], "operationType"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.OperationTypeAdditionalInfo, err = optionalString(payload["operationTypeAdditionalInfo"], "operationTypeAdditionalInfo"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.OperationCategory, err = optionalString(payload["operationCategory"], "operationCategory"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.ProviderID, err = optionalString(payload["providerId"], "providerId"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.ProviderOrder, err = optionalInteger(payload["order"], "order"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.ProviderCreatedAt, err = optionalDateTime(payload["createdAt"], "createdAt"); err != nil {
		return TransactionSnapshot{}, err
	}
	if t.ProviderUpdatedAt, err = optionalDateTime(payload["updatedAt"], "updatedAt"); err != nil {
		return TransactionSnapshot{}, err
	}
	return t, nil
}

func mapAccountsPage(payload map[string]any, institution *string) ([]AccountSnapshot, []RejectedRecord, error) {
	results, ok := payload["results"].([]any)
	if !ok {
		return nil, nil, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Provider results must be a list"}
	}
	var accounts []AccountSnapshot
	var rejections []RejectedRecord
	for _, raw := range results {
		record, ok := raw.(map[string]any)
		if !ok {
			rejections = append(rejections, RejectedRecord{EntityType: "account", Code: "invalid_provider_payload", SafeMessage: "Provider record must be an object"})
			continue
		}
		account, err := mapAccount(record, institution)
		if err != nil {
			rejections = append(rejections, toRejection("account", record, err))
			continue
		}
		accounts = append(accounts, account)
	}
	return accounts, rejections, nil
}

func mapTransactionsPage(payload map[string]any, expectedAccountID string) ([]TransactionSnapshot, []RejectedRecord, error) {
	results, ok := payload["results"].([]any)
	if !ok {
		return nil, nil, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Provider results must be a list"}
	}
	var transactions []TransactionSnapshot
	var rejections []RejectedRecord
	for _, raw := range results {
		record, ok := raw.(map[string]any)
		if !ok {
			rejections = append(rejections, RejectedRecord{EntityType: "transaction", Code: "invalid_provider_payload", SafeMessage: "Provider record must be an object"})
			continue
		}
		transaction, err := mapTransaction(record, expectedAccountID)
		if err != nil {
			rejections = append(rejections, toRejection("transaction", record, err))
			continue
		}
		transactions = append(transactions, transaction)
	}
	return transactions, rejections, nil
}

func toRejection(entityType string, record map[string]any, err error) RejectedRecord {
	mapErr, _ := err.(*MappingError)
	externalID := mapErr.ExternalID
	if externalID == nil {
		if id, ok := record["id"].(string); ok {
			externalID = &id
		}
	}
	return RejectedRecord{EntityType: entityType, ExternalID: externalID, Code: mapErr.Code, SafeMessage: mapErr.SafeMessage}
}

// extractCursor mirrors _extract_cursor: Pluggy's "next" link carries the
// pagination cursor as an "after" query parameter, which must stay scoped to
// the account being paginated.
func extractCursor(value any, expectedAccountID string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	link, ok := value.(string)
	if !ok {
		return nil, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Pagination cursor is invalid"}
	}
	parsed, err := url.Parse(link)
	if err != nil {
		return nil, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Pagination cursor is invalid"}
	}
	query := parsed.Query()
	if accounts, ok := query["accountId"]; ok && !(len(accounts) == 1 && accounts[0] == expectedAccountID) {
		return nil, &MappingError{Code: "unsafe_account_association", SafeMessage: "Pagination changed account context"}
	}
	cursors := query["after"]
	if len(cursors) != 1 || cursors[0] == "" {
		return nil, &MappingError{Code: "invalid_provider_payload", SafeMessage: "Pagination cursor is missing"}
	}
	return &cursors[0], nil
}
