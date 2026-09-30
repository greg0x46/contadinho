package httpapi

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"contadinho-go/internal/db"
)

const validFlash = "\ufeffData,Hora,Movimentação,Valor,Meio de Pagamento,Saldo\n09/09/2026,12:00,Café,\"-R$ 10,00\",Cartão,\"R$ 90,00\"\n08/09/2026,12:00,Depósito,\"R$ 100,00\",Depósito,\"R$ 100,00\"\n"

func uploadStatement(t *testing.T, handler http.HandlerFunc, file string, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	f, e := w.CreateFormFile("file", "flash_06-2026.csv")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.Write([]byte(file)); e != nil {
		t.Fatal(e)
	}
	for k, v := range fields {
		if e = w.WriteField(k, v); e != nil {
			t.Fatal(e)
		}
	}
	w.Close()
	r := httptest.NewRequest("POST", "/api/statement-imports", &b)
	r.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	handler(rr, r)
	return rr
}
func TestStatementImportRoundTrip(t *testing.T) {
	conn, e := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	preview := uploadStatement(t, handleStatementPreview(conn), validFlash, nil)
	if preview.Code != 200 {
		t.Fatalf("preview %d: %s", preview.Code, preview.Body.String())
	}
	var p struct {
		Format             string       `json:"format"`
		FormatVersion      string       `json:"format_version"`
		SHA256             string       `json:"sha256"`
		PreviewFingerprint string       `json:"preview_fingerprint"`
		PeriodStart        string       `json:"period_start"`
		Counts             importCounts `json:"counts"`
	}
	if e = json.Unmarshal(preview.Body.Bytes(), &p); e != nil {
		t.Fatal(e)
	}
	if p.PeriodStart != "2026-09-08" || p.Counts.New != 2 || p.Counts.Invalid != 0 {
		t.Fatalf("bad preview: %+v", p)
	}
	fields := map[string]string{"new_account_name": "Flash refeições", "expected_sha256": p.SHA256, "expected_format": p.Format, "expected_format_version": p.FormatVersion, "expected_preview_fingerprint": p.PreviewFingerprint}
	confirm := uploadStatement(t, handleStatementConfirm(conn), validFlash, fields)
	if confirm.Code != 201 {
		t.Fatalf("confirm %d: %s", confirm.Code, confirm.Body.String())
	}
	var result struct {
		AccountID string       `json:"account_id"`
		Counts    importCounts `json:"counts"`
	}
	if e = json.Unmarshal(confirm.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	if result.Counts.New != 2 {
		t.Fatalf("bad count %+v", result.Counts)
	}
	var balance string
	if e = conn.QueryRow(`SELECT balance FROM financial_accounts WHERE id=?`, result.AccountID).Scan(&balance); e != nil || balance != "90.00" {
		t.Fatalf("balance %s %v", balance, e)
	}
	// The frontend only accepts BANK or CREDIT: anything else breaks every
	// screen that lists accounts.
	var accountType string
	if e = conn.QueryRow(`SELECT account_type FROM financial_accounts WHERE id=?`, result.AccountID).Scan(&accountType); e != nil || accountType != "BANK" {
		t.Fatalf("account_type %s %v", accountType, e)
	}
	fields = map[string]string{"account_id": result.AccountID}
	secondPreview := uploadStatement(t, handleStatementPreview(conn), validFlash, fields)
	if secondPreview.Code != 200 {
		t.Fatalf("second preview %d: %s", secondPreview.Code, secondPreview.Body.String())
	}
	if e = json.Unmarshal(secondPreview.Body.Bytes(), &p); e != nil {
		t.Fatal(e)
	}
	if p.Counts.Duplicate != 2 {
		t.Fatalf("reimport preview %+v", p.Counts)
	}
	fields["expected_sha256"] = p.SHA256
	fields["expected_format"] = p.Format
	fields["expected_format_version"] = p.FormatVersion
	fields["expected_preview_fingerprint"] = p.PreviewFingerprint
	second := uploadStatement(t, handleStatementConfirm(conn), validFlash, fields)
	if second.Code != 201 {
		t.Fatalf("second confirm %d: %s", second.Code, second.Body.String())
	}
	if e = json.Unmarshal(second.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	if result.Counts.New != 0 || result.Counts.Duplicate != 2 {
		t.Fatalf("second counts %+v", result.Counts)
	}
	// A same-looking manual line remains separate and is surfaced for review.
	manualID := uuid.NewString()
	_, e = conn.Exec(`INSERT INTO financial_transactions (id,account_id,description,amount,amount_in_account_currency,currency_code,occurred_at,origin,created_at,updated_at) VALUES (?,?,?,'-10.00','-10.00','BRL','2026-09-09T15:00:00.000000000Z','manual','2026-09-09T15:00:00.000000000Z','2026-09-09T15:00:00.000000000Z')`, manualID, result.AccountID, "Café")
	if e != nil {
		t.Fatal(e)
	}
	alternate := strings.Replace(validFlash, "Cartão,\"R$ 90,00\"", "PIX,\"R$ 90,00\"", 1)
	warn := uploadStatement(t, handleStatementPreview(conn), alternate, map[string]string{"account_id": result.AccountID})
	if warn.Code != 200 || !strings.Contains(warn.Body.String(), "possível correspondência com lançamento manual") {
		t.Fatalf("manual warning %d: %s", warn.Code, warn.Body.String())
	}
}
