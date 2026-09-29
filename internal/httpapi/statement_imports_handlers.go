package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
	"contadinho-go/internal/statementimport"
)

type importCounts struct {
	Total     int `json:"total"`
	New       int `json:"new"`
	Duplicate int `json:"duplicate"`
	Invalid   int `json:"invalid"`
}
type importPreview struct {
	statementimport.Parsed
	Filename           string       `json:"filename"`
	Counts             importCounts `json:"counts"`
	PreviewFingerprint string       `json:"preview_fingerprint"`
}
type importAccount struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CurrencyCode string `json:"currency_code"`
}
type importHistory struct {
	RunID       string       `json:"run_id"`
	AccountID   string       `json:"account_id"`
	AccountName string       `json:"account_name"`
	Filename    string       `json:"filename"`
	Format      string       `json:"format"`
	CreatedAt   string       `json:"created_at"`
	Counts      importCounts `json:"counts"`
}

func readStatementUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, statementimport.MaxBytes+1<<16)
	if err := r.ParseMultipartForm(statementimport.MaxBytes + 1<<16); err != nil {
		writeProblem(w, 413, "upload-too-large", "Arquivo muito grande", "O limite é 2 MB.")
		return nil, "", false
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		writeProblem(w, 422, "missing-file", "Arquivo ausente", "Selecione um CSV.")
		return nil, "", false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, statementimport.MaxBytes+1))
	if err != nil || len(data) > statementimport.MaxBytes {
		writeProblem(w, 413, "upload-too-large", "Arquivo muito grande", "O limite é 2 MB.")
		return nil, "", false
	}
	return data, h.Filename, true
}
func loadFileAccount(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (string, error) {
	var source string
	err := q.QueryRowContext(ctx, `SELECT fa.source_id FROM financial_accounts fa JOIN data_sources ds ON ds.id=fa.source_id WHERE fa.id=? AND ds.provider='file' AND fa.currency_code='BRL'`, id).Scan(&source)
	return source, err
}
func digest(parts ...string) string {
	s := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(s[:])
}
func classifyStatement(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, p *importPreview, accountID string) error {
	seen := map[string]int{}
	p.Counts = importCounts{Total: len(p.Rows)}
	for i := range p.Rows {
		row := &p.Rows[i]
		if row.Status == "invalid" {
			p.Counts.Invalid++
			continue
		}
		normalized := strings.ToLower(strings.Join(strings.Fields(row.Description), " "))
		base := digest(row.OccurredAt, normalized, row.Amount, strings.ToLower(row.PaymentMethod))
		seen[base]++
		row.Identity = digest("statement-v1", accountID, base, fmt.Sprint(seen[base]))
		row.ContentHash = digest(row.Identity, row.Balance)
		row.Status = "new"
		if accountID != "" {
			var oldBalance sql.NullString
			err := q.QueryRowContext(ctx, `SELECT balance_after FROM financial_transactions WHERE account_id=? AND external_id=?`, accountID, row.Identity).Scan(&oldBalance)
			if err == nil {
				row.Status = "duplicate"
				if oldBalance.Valid && oldBalance.String != row.Balance {
					row.Warnings = append(row.Warnings, "saldo difere da importação anterior; revisão necessária")
				}
			} else if err != sql.ErrNoRows {
				return err
			}
		}
		if row.Status == "duplicate" {
			p.Counts.Duplicate++
		} else {
			p.Counts.New++
			var similarPluggy int
			err := q.QueryRowContext(ctx, `SELECT 1 FROM financial_transactions ft JOIN data_sources ds ON ds.id=ft.source_id WHERE ds.provider='pluggy' AND ft.deleted_at IS NULL AND ft.occurred_at=? AND ft.amount=? AND LOWER(TRIM(ft.description))=LOWER(TRIM(?)) LIMIT 1`, row.OccurredAt, row.Amount, row.Description).Scan(&similarPluggy)
			if err == nil {
				row.Warnings = append(row.Warnings, "possível correspondência com conexão automática")
			} else if err != sql.ErrNoRows {
				return err
			}
			if accountID != "" {
				var similar int
				err = q.QueryRowContext(ctx, `SELECT 1 FROM financial_transactions WHERE account_id=? AND origin='manual' AND deleted_at IS NULL AND occurred_at=? AND amount=? AND LOWER(TRIM(description))=LOWER(TRIM(?)) LIMIT 1`, accountID, row.OccurredAt, row.Amount, row.Description).Scan(&similar)
				if err == nil {
					row.Warnings = append(row.Warnings, "possível correspondência com lançamento manual")
				} else if err != sql.ErrNoRows {
					return err
				}
			}
		}
	}
	// Sort by timestamp and compare only unambiguous adjacent movements.
	order := make([]int, 0, len(p.Rows))
	states := make([]string, 0, len(p.Rows))
	for i := range p.Rows {
		states = append(states, p.Rows[i].Status+"|"+strings.Join(p.Rows[i].Warnings, "|"))
		if p.Rows[i].Status != "invalid" {
			order = append(order, i)
		}
	}
	p.PreviewFingerprint = digest(append([]string{accountID}, states...)...)
	sort.Slice(order, func(i, j int) bool { return p.Rows[order[i]].OccurredAt < p.Rows[order[j]].OccurredAt })
	for i := 1; i < len(order); i++ {
		prev := &p.Rows[order[i-1]]
		next := &p.Rows[order[i]]
		if prev.OccurredAt == next.OccurredAt || (i+1 < len(order) && p.Rows[order[i+1]].OccurredAt == next.OccurredAt) || (i > 1 && p.Rows[order[i-2]].OccurredAt == prev.OccurredAt) {
			continue
		}
		ab, e1 := decimal.NewFromString(next.Balance)
		bb, e2 := decimal.NewFromString(prev.Balance)
		amount, e3 := decimal.NewFromString(next.Amount)
		if e1 == nil && e2 == nil && e3 == nil && !ab.Sub(bb).Equal(amount) {
			next.Warnings = append(next.Warnings, "continuidade do saldo não confere")
		}
	}

	return nil
}
func makeStatementPreview(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, data []byte, filename, format, accountID string) (importPreview, error) {
	parsed, err := statementimport.Parse(data, format)
	if err != nil {
		return importPreview{}, err
	}
	p := importPreview{Parsed: parsed, Filename: filename}
	if err := classifyStatement(ctx, q, &p, accountID); err != nil {
		return importPreview{}, err
	}
	return p, nil
}
func handleStatementPreview(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, name, ok := readStatementUpload(w, r)
		if !ok {
			return
		}
		accountID := strings.TrimSpace(r.FormValue("account_id"))
		if accountID != "" {
			if _, err := loadFileAccount(r.Context(), conn, accountID); err != nil {
				writeProblem(w, 422, "invalid-account", "Conta inválida", "Escolha uma conta de arquivo em BRL.")
				return
			}
		}
		p, err := makeStatementPreview(r.Context(), conn, data, name, r.FormValue("format"), accountID)
		if err != nil {
			writeProblem(w, 422, "invalid-statement", "Extrato inválido", err.Error())
			return
		}
		writeJSON(w, 200, p)
	}
}
func handleStatementAccounts(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.QueryContext(r.Context(), `SELECT fa.id, COALESCE(fa.name,''), fa.currency_code FROM financial_accounts fa JOIN data_sources ds ON ds.id=fa.source_id WHERE ds.provider='file' AND fa.currency_code='BRL' ORDER BY fa.name,fa.id`)
		if err != nil {
			writeProblem(w, 503, "accounts-unavailable", "Contas indisponíveis", "")
			return
		}
		defer rows.Close()
		out := []importAccount{}
		for rows.Next() {
			var a importAccount
			if err := rows.Scan(&a.ID, &a.Name, &a.CurrencyCode); err != nil {
				writeProblem(w, 503, "accounts-unavailable", "Contas indisponíveis", "")
				return
			}
			out = append(out, a)
		}
		if rows.Err() != nil {
			writeProblem(w, 503, "accounts-unavailable", "Contas indisponíveis", "")
			return
		}
		writeJSON(w, 200, out)
	}
}
func handleStatementHistory(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.QueryContext(r.Context(), `SELECT si.sync_run_id,si.account_id,COALESCE(fa.name,''),si.filename,si.format,si.created_at,si.rows_total,sr.transactions_inserted,si.rows_duplicate,si.rows_invalid FROM statement_imports si JOIN sync_runs sr ON sr.id=si.sync_run_id JOIN financial_accounts fa ON fa.id=si.account_id ORDER BY si.created_at DESC LIMIT 100`)
		if err != nil {
			writeProblem(w, 503, "imports-unavailable", "Importações indisponíveis", "")
			return
		}
		defer rows.Close()
		out := []importHistory{}
		for rows.Next() {
			var h importHistory
			if err := rows.Scan(&h.RunID, &h.AccountID, &h.AccountName, &h.Filename, &h.Format, &h.CreatedAt, &h.Counts.Total, &h.Counts.New, &h.Counts.Duplicate, &h.Counts.Invalid); err != nil {
				writeProblem(w, 503, "imports-unavailable", "Importações indisponíveis", "")
				return
			}
			out = append(out, h)
		}
		if rows.Err() != nil {
			writeProblem(w, 503, "imports-unavailable", "Importações indisponíveis", "")
			return
		}
		writeJSON(w, 200, out)
	}
}
func handleStatementConfirm(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, name, ok := readStatementUpload(w, r)
		if !ok {
			return
		}
		accountID := strings.TrimSpace(r.FormValue("account_id"))
		newName := strings.TrimSpace(r.FormValue("new_account_name"))
		if (accountID == "") == (newName == "") || len(newName) > 120 {
			writeProblem(w, 422, "invalid-account", "Conta inválida", "Escolha uma conta ou informe um nome para a nova conta.")
			return
		}
		if r.FormValue("expected_sha256") == "" || r.FormValue("expected_format") == "" || r.FormValue("expected_format_version") == "" || r.FormValue("expected_preview_fingerprint") == "" {
			writeProblem(w, 422, "missing-preview", "Prévia necessária", "Confira a prévia antes de importar.")
			return
		}
		tx, err := conn.BeginTx(r.Context(), nil)
		if err != nil {
			writeProblem(w, 503, "import-unavailable", "Importação indisponível", "")
			return
		}
		defer tx.Rollback()
		sourceID := ""
		if accountID != "" {
			sourceID, err = loadFileAccountTx(r.Context(), tx, accountID)
			if err != nil {
				writeProblem(w, 422, "invalid-account", "Conta inválida", "Escolha uma conta de arquivo em BRL.")
				return
			}
		}
		p, err := makeStatementPreviewTx(r.Context(), tx, data, name, r.FormValue("format"), accountID)
		if err != nil {
			writeProblem(w, 422, "invalid-statement", "Extrato inválido", err.Error())
			return
		}
		if p.SHA256 != r.FormValue("expected_sha256") || p.Format != r.FormValue("expected_format") || p.FormatVersion != r.FormValue("expected_format_version") {
			writeProblem(w, 409, "preview-changed", "Arquivo alterado", "Gere uma nova prévia antes de importar.")
			return
		}
		if p.PreviewFingerprint != r.FormValue("expected_preview_fingerprint") {
			writeProblem(w, 409, "preview-changed", "Prévia alterada", "As movimentações da conta mudaram. Gere uma nova prévia.")
			return
		}
		if p.Counts.New == 0 && newName != "" {
			writeProblem(w, 422, "empty-import", "Nada para importar", "O extrato não contém movimentações válidas.")
			return
		}
		if p.Counts.Invalid > 0 && r.FormValue("allow_partial") != "true" {
			writeProblem(w, 422, "invalid-rows", "Linhas inválidas", "Revise as linhas ou autorize importar somente as válidas.")
			return
		}
		now := db.FormatTime(time.Now().UTC())
		runID := uuid.NewString()
		rawID := uuid.NewString()
		if accountID == "" {
			sourceID = uuid.NewString()
			accountID = uuid.NewString()
			_, err = tx.ExecContext(r.Context(), `INSERT INTO data_sources (id,provider,external_item_id,display_name,label,created_at,updated_at) VALUES (?,'file',?,?,?,?,?)`, sourceID, sourceID, newName, newName, now, now)
			if err != nil {
				importFailure(w)
				return
			}
			if err = classifyStatementTx(r.Context(), tx, &p, accountID); err != nil {
				importFailure(w)
				return
			}
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO sync_runs (id,source_id,run_type,status,started_at,finished_at,accounts_processed,transactions_inserted) VALUES (?,?,'file_import','completed',?,?,1,?)`, runID, sourceID, now, now, p.Counts.New); err != nil {
			importFailure(w)
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO raw_imports (id,sync_run_id,source_id,scope,external_account_id,page_sequence,request_attempt,payload,payload_sha256,received_at) VALUES (?,?,?,'file',?,1,1,?,?,?)`, rawID, runID, sourceID, accountID, data, p.SHA256, now); err != nil {
			importFailure(w)
			return
		}
		if newName != "" {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO financial_accounts (id,source_id,external_id,institution,name,account_type,account_subtype,currency_code,current_raw_import_id,normalized_hash,created_at,updated_at) VALUES (?,?,?,?,?,'BANK','CHECKING_ACCOUNT','BRL',?,?,?,?)`, accountID, sourceID, accountID, p.Institution, newName, rawID, digest(accountID, newName), now, now)
			if err != nil {
				importFailure(w)
				return
			}
		}
		latestTime := ""
		latestBalance := ""
		inserted := 0
		duplicates := 0
		for _, row := range p.Rows {
			if row.Status == "invalid" {
				continue
			}
			if row.OccurredAt > latestTime {
				latestTime = row.OccurredAt
				latestBalance = row.Balance
				for _, warning := range row.Warnings {
					if strings.Contains(warning, "saldo difere da importação anterior") {
						latestBalance = ""
					}
				}
			}
			if row.Status == "duplicate" {
				duplicates++
				continue
			}
			id := uuid.NewString()
			move := "DEBIT"
			if strings.HasPrefix(row.Amount, "-") == false {
				move = "CREDIT"
			}
			res, e := tx.ExecContext(r.Context(), `INSERT INTO financial_transactions (id,source_id,account_id,external_id,description,description_raw,amount,amount_in_account_currency,balance_after,currency_code,occurred_at,provider_status,movement_type,source_category,current_raw_import_id,normalized_hash,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?, 'BRL',?,'POSTED',?,?,?,?,?,?) ON CONFLICT (source_id,external_id) DO NOTHING`, id, sourceID, accountID, row.Identity, row.Description, row.Description, row.Amount, row.Amount, row.Balance, row.OccurredAt, move, row.PaymentMethod, rawID, row.ContentHash, now, now)
			if e != nil {
				importFailure(w)
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				writeProblem(w, 409, "preview-changed", "Prévia alterada", "As movimentações da conta mudaram. Gere uma nova prévia.")
				return
			}
			inserted++
			if _, e = tx.ExecContext(r.Context(), `INSERT INTO normalization_events (id,sync_run_id,raw_import_id,entity_type,transaction_id,external_id,outcome,normalized_hash,created_at) VALUES (?,?,?,'transaction',?,?,'inserted',?,?)`, uuid.NewString(), runID, rawID, id, row.Identity, row.ContentHash, now); e != nil {
				importFailure(w)
				return
			}
			if _, e = categories.ApplyLearned(r.Context(), tx, id); e != nil {
				importFailure(w)
				return
			}
			if e = automation.ApplyToNewTransactionWithQuerier(r.Context(), tx, id, onIgnoredHook); e != nil {
				importFailure(w)
				return
			}
		}
		latestCount := 0
		for _, row := range p.Rows {
			if row.Status != "invalid" && row.OccurredAt == latestTime {
				latestCount++
			}
		}
		if latestCount > 1 || latestBalance == "" {
			latestTime = ""
		}
		if latestTime != "" {
			_, err = tx.ExecContext(r.Context(), `UPDATE financial_accounts SET balance=?,balance_as_of=?,updated_at=? WHERE id=? AND (balance_as_of IS NULL OR balance_as_of < ?)`, latestBalance, latestTime, now, accountID, latestTime)
			if err != nil {
				importFailure(w)
				return
			}
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE sync_runs SET transactions_inserted=? WHERE id=?`, inserted, runID); err != nil {
			importFailure(w)
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO statement_imports (id,sync_run_id,source_id,account_id,format,adapter_version,filename,file_sha256,rows_total,rows_invalid,rows_duplicate,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, uuid.NewString(), runID, sourceID, accountID, p.Format, p.FormatVersion, name, p.SHA256, p.Counts.Total, p.Counts.Invalid, duplicates, now); err != nil {
			importFailure(w)
			return
		}
		if err = tx.Commit(); err != nil {
			importFailure(w)
			return
		}
		writeJSON(w, 201, map[string]any{"run_id": runID, "account_id": accountID, "counts": importCounts{Total: p.Counts.Total, New: inserted, Duplicate: duplicates, Invalid: p.Counts.Invalid}})
	}
}
func importFailure(w http.ResponseWriter) {
	writeProblem(w, 503, "import-unavailable", "Importação indisponível", "Tente novamente em instantes.")
}

// sql.Tx.QueryRowContext returns *sql.Row too; these wrappers keep the
// preview and confirmation on the same database transaction.
func loadFileAccountTx(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var s string
	err := tx.QueryRowContext(ctx, `SELECT fa.source_id FROM financial_accounts fa JOIN data_sources ds ON ds.id=fa.source_id WHERE fa.id=? AND ds.provider='file' AND fa.currency_code='BRL'`, id).Scan(&s)
	return s, err
}
func makeStatementPreviewTx(ctx context.Context, tx *sql.Tx, data []byte, name, format, account string) (importPreview, error) {
	p, e := statementimport.Parse(data, format)
	if e != nil {
		return importPreview{}, e
	}
	v := importPreview{Parsed: p, Filename: name}
	e = classifyStatementTx(ctx, tx, &v, account)
	return v, e
}
func classifyStatementTx(ctx context.Context, tx *sql.Tx, p *importPreview, account string) error {
	return classifyStatement(ctx, txQuery{tx}, p, account)
}

type txQuery struct{ *sql.Tx }

func (t txQuery) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	return t.Tx.QueryRowContext(ctx, q, args...)
}
