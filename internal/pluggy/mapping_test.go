package pluggy

import (
	"testing"
)

func TestDecodeJSONPreservesDecimalPrecision(t *testing.T) {
	payload, err := decodeJSON([]byte(`{"amount": 10.10, "id": "x"}`))
	if err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
	d, err := optionalDecimal(payload["amount"], "amount")
	if err != nil {
		t.Fatalf("optionalDecimal: %v", err)
	}
	if d.String() != "10.1" { // shopspring trims trailing zeros in String(), but exponent preserves scale
		t.Errorf("amount = %s", d.String())
	}
	if got := d.StringFixed(2); got != "10.10" {
		t.Errorf("StringFixed(2) = %s, want 10.10 (precision lost)", got)
	}
}

func TestDecodeJSONRejectsNonObjectRoot(t *testing.T) {
	if _, err := decodeJSON([]byte(`[1,2,3]`)); err == nil {
		t.Fatal("expected an error for a non-object root")
	}
	if _, err := decodeJSON([]byte(`not json`)); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestMapSourceRejectsMismatchedItemID(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{"id": "other-item", "status": "UPDATED"}`))
	if _, err := mapSource(payload, "expected-item"); err == nil {
		t.Fatal("expected an error for mismatched item id")
	}
}

func TestMapSourceExtractsInstitutionFromConnector(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"id": "item-1", "status": "UPDATED", "executionStatus": "SUCCESS",
		"connector": {"name": "Banco Exemplo"}, "lastUpdatedAt": "2026-01-01T00:00:00Z"
	}`))
	source, err := mapSource(payload, "item-1")
	if err != nil {
		t.Fatalf("mapSource: %v", err)
	}
	if source.InstitutionName == nil || *source.InstitutionName != "Banco Exemplo" {
		t.Errorf("InstitutionName = %v", source.InstitutionName)
	}
	if source.ProviderUpdatedAt == nil {
		t.Error("ProviderUpdatedAt should be set")
	}
}

func TestMapAccountExtractsCreditLimitFromCreditData(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"id": "acc-1", "name": "Cartão", "creditData": {"creditLimit": 5000.00}, "currencyCode": "BRL"
	}`))
	institution := "Banco Exemplo"
	account, err := mapAccount(payload, &institution)
	if err != nil {
		t.Fatalf("mapAccount: %v", err)
	}
	if account.CreditLimit == nil || account.CreditLimit.StringFixed(2) != "5000.00" {
		t.Errorf("CreditLimit = %v", account.CreditLimit)
	}
	if account.Institution == nil || *account.Institution != institution {
		t.Errorf("Institution = %v", account.Institution)
	}
}

func TestMapAccountRejectsMissingID(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{"name": "no id"}`))
	if _, err := mapAccount(payload, nil); err == nil {
		t.Fatal("expected an error for a missing account id")
	}
}

func TestMapTransactionRejectsAccountMismatch(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{"id": "tx-1", "accountId": "acc-2", "amount": -10.00}`))
	_, err := mapTransaction(payload, "acc-1")
	if err == nil {
		t.Fatal("expected an error for account mismatch")
	}
	mapErr, ok := err.(*MappingError)
	if !ok || mapErr.Code != "unsafe_account_association" {
		t.Errorf("err = %v, want unsafe_account_association", err)
	}
}

func TestMapTransactionMapsCoreFields(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"id": "tx-1", "accountId": "acc-1", "description": "Mercado", "amount": -42.50,
		"amountInAccountCurrency": -42.50, "currencyCode": "BRL", "date": "2026-03-15T10:00:00Z",
		"status": "POSTED", "type": "DEBIT", "category": "Groceries", "order": 3,
		"merchant": {"name": "Mercado XYZ"}
	}`))
	tx, err := mapTransaction(payload, "acc-1")
	if err != nil {
		t.Fatalf("mapTransaction: %v", err)
	}
	if tx.Amount == nil || tx.Amount.StringFixed(2) != "-42.50" {
		t.Errorf("Amount = %v", tx.Amount)
	}
	if tx.MovementType == nil || *tx.MovementType != "DEBIT" {
		t.Errorf("MovementType = %v", tx.MovementType)
	}
	if tx.ProviderOrder == nil || *tx.ProviderOrder != 3 {
		t.Errorf("ProviderOrder = %v", tx.ProviderOrder)
	}
	if tx.Merchant == nil {
		t.Error("Merchant should be captured as raw JSON")
	}
}

func TestMapTransactionsPageCollectsRejectionsWithoutFailingWholePage(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"results": [
			{"id": "tx-1", "accountId": "acc-1", "amount": -1.00},
			{"id": "tx-2", "accountId": "wrong-account", "amount": -2.00},
			{"notAnObject": true}
		]
	}`))
	transactions, rejections, err := mapTransactionsPage(payload, "acc-1")
	if err != nil {
		t.Fatalf("mapTransactionsPage: %v", err)
	}
	if len(transactions) != 1 {
		t.Errorf("len(transactions) = %d, want 1", len(transactions))
	}
	if len(rejections) != 2 {
		t.Errorf("len(rejections) = %d, want 2", len(rejections))
	}
}

func TestMapInvestmentMapsCoreFields(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"id": "inv-1", "type": "MUTUAL_FUND", "subtype": "MULTIMARKET", "name": "Fundo XYZ",
		"balance": 1000.50, "currencyCode": "BRL", "quantity": 10.5, "value": 95.28,
		"amount": 1000.50, "date": "2026-03-15T10:00:00Z", "lastUpdatedAt": "2026-03-16T10:00:00Z"
	}`))
	investment, err := mapInvestment(payload)
	if err != nil {
		t.Fatalf("mapInvestment: %v", err)
	}
	if investment.InvestmentType == nil || *investment.InvestmentType != "MUTUAL_FUND" {
		t.Errorf("InvestmentType = %v", investment.InvestmentType)
	}
	if investment.Balance == nil || investment.Balance.StringFixed(2) != "1000.50" {
		t.Errorf("Balance = %v", investment.Balance)
	}
	if investment.AsOfDate == nil {
		t.Error("AsOfDate should be captured")
	}
}

func TestMapInvestmentRejectsMissingID(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{"name": "no id"}`))
	if _, err := mapInvestment(payload); err == nil {
		t.Fatal("expected an error for a missing investment id")
	}
}

func TestMapInvestmentTransactionRejectsInvestmentMismatch(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{"id": "invtx-1", "investmentId": "inv-2", "amount": -10.00}`))
	_, err := mapInvestmentTransaction(payload, "inv-1")
	if err == nil {
		t.Fatal("expected an error for investment mismatch")
	}
	mapErr, ok := err.(*MappingError)
	if !ok || mapErr.Code != "unsafe_investment_association" {
		t.Errorf("err = %v, want unsafe_investment_association", err)
	}
}

func TestMapInvestmentTransactionMapsCoreFields(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"id": "invtx-1", "investmentId": "inv-1", "type": "BUY", "quantity": 5.0, "value": 100.00,
		"amount": 500.00, "date": "2026-03-15T10:00:00Z", "tradeDate": "2026-03-14T10:00:00Z"
	}`))
	tx, err := mapInvestmentTransaction(payload, "inv-1")
	if err != nil {
		t.Fatalf("mapInvestmentTransaction: %v", err)
	}
	if tx.MovementType == nil || *tx.MovementType != "BUY" {
		t.Errorf("MovementType = %v", tx.MovementType)
	}
	if tx.Amount == nil || tx.Amount.StringFixed(2) != "500.00" {
		t.Errorf("Amount = %v", tx.Amount)
	}
}

// Pluggy's real investment-transactions responses carry no "investmentId"
// field at all (the endpoint is already scoped to one investment), so the
// association must come from expectedInvestmentID rather than fail as a
// missing required field.
func TestMapInvestmentTransactionAcceptsMissingInvestmentID(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"id": "invtx-1", "type": "BUY", "quantity": 5.0, "value": 100.00,
		"amount": 500.00, "date": "2026-03-15T10:00:00Z", "tradeDate": "2026-03-14T10:00:00Z"
	}`))
	tx, err := mapInvestmentTransaction(payload, "inv-1")
	if err != nil {
		t.Fatalf("mapInvestmentTransaction: %v", err)
	}
	if tx.ExternalInvestmentID != "inv-1" {
		t.Errorf("ExternalInvestmentID = %v, want inv-1", tx.ExternalInvestmentID)
	}
}

func TestMapInvestmentsPageCollectsRejectionsWithoutFailingWholePage(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"results": [
			{"id": "inv-1", "balance": 100.00},
			{"notAnObject": true}
		]
	}`))
	investments, rejections, err := mapInvestmentsPage(payload)
	if err != nil {
		t.Fatalf("mapInvestmentsPage: %v", err)
	}
	if len(investments) != 1 {
		t.Errorf("len(investments) = %d, want 1", len(investments))
	}
	if len(rejections) != 1 {
		t.Errorf("len(rejections) = %d, want 1", len(rejections))
	}
}

func TestMapInvestmentTransactionsPageCollectsRejectionsWithoutFailingWholePage(t *testing.T) {
	payload, _ := decodeJSON([]byte(`{
		"results": [
			{"id": "invtx-1", "investmentId": "inv-1", "amount": -1.00},
			{"id": "invtx-2", "investmentId": "wrong-investment", "amount": -2.00},
			{"notAnObject": true}
		]
	}`))
	transactions, rejections, err := mapInvestmentTransactionsPage(payload, "inv-1")
	if err != nil {
		t.Fatalf("mapInvestmentTransactionsPage: %v", err)
	}
	if len(transactions) != 1 {
		t.Errorf("len(transactions) = %d, want 1", len(transactions))
	}
	if len(rejections) != 2 {
		t.Errorf("len(rejections) = %d, want 2", len(rejections))
	}
}

func TestExtractCursorValidatesAccountScope(t *testing.T) {
	cursor, err := extractCursor("https://api.pluggy.ai/v2/transactions?accountId=acc-1&after=abc", "acc-1")
	if err != nil {
		t.Fatalf("extractCursor: %v", err)
	}
	if cursor == nil || *cursor != "abc" {
		t.Errorf("cursor = %v, want abc", cursor)
	}

	if _, err := extractCursor("https://api.pluggy.ai/v2/transactions?accountId=other&after=abc", "acc-1"); err == nil {
		t.Fatal("expected an error when the cursor changes account context")
	}

	if cursor, err := extractCursor(nil, "acc-1"); err != nil || cursor != nil {
		t.Errorf("nil cursor: cursor=%v err=%v", cursor, err)
	}
}
