package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
	"contadinho-go/internal/settings"
)

var resultMessages = map[string]string{
	"in_progress":             "Synchronization is in progress.",
	"completed":               "Synchronization completed successfully.",
	"completed_with_failures": "Synchronization completed with retained partial failures.",
	"failed":                  "Synchronization failed; previously imported data was preserved.",
}

type syncRunDTO struct {
	ID                   string     `json:"id"`
	Status               string     `json:"status"`
	StartedAt            time.Time  `json:"started_at"`
	FinishedAt           *time.Time `json:"finished_at"`
	AccountsProcessed    int        `json:"accounts_processed"`
	TransactionsInserted int        `json:"transactions_inserted"`
	TransactionsUpdated  int        `json:"transactions_updated"`
	ResultMessage        string     `json:"result_message"`
}

type syncFailureDTO struct {
	Stage                 string    `json:"stage"`
	Code                  string    `json:"code"`
	Message               string    `json:"message"`
	ExternalAccountID     *string   `json:"external_account_id"`
	ExternalTransactionID *string   `json:"external_transaction_id"`
	OccurredAt            time.Time `json:"occurred_at"`
}

func scanSyncRun(row *sql.Row) (syncRunDTO, error) {
	var (
		d             syncRunDTO
		startedAtRaw  string
		finishedAtRaw sql.NullString
	)
	if err := row.Scan(&d.ID, &d.Status, &startedAtRaw, &finishedAtRaw,
		&d.AccountsProcessed, &d.TransactionsInserted, &d.TransactionsUpdated); err != nil {
		return syncRunDTO{}, err
	}
	started, err := db.ParseTime(startedAtRaw)
	if err != nil {
		return syncRunDTO{}, err
	}
	d.StartedAt = started
	if d.FinishedAt, err = db.ParseNullTime(finishedAtRaw); err != nil {
		return syncRunDTO{}, err
	}
	d.ResultMessage = resultMessages[d.Status]
	return d, nil
}

func handleCreateSyncRun(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID, found, err := settings.Get(r.Context(), conn, "pluggy.item_id", nil)
		if err != nil {
			writeProblem(w, 503, "sync-run-unavailable", "Sincronização temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		if !found {
			writeProblem(w, 409, "not-configured", "Aplicação ainda não configurada", "Complete a configuração inicial antes de sincronizar.")
			return
		}

		var sourceID string
		err = conn.QueryRowContext(r.Context(),
			`SELECT id FROM data_sources WHERE provider = 'pluggy' AND external_item_id = ?`, itemID,
		).Scan(&sourceID)
		if errors.Is(err, sql.ErrNoRows) {
			sourceID = uuid.NewString()
			now := db.FormatTime(time.Now())
			_, err = conn.ExecContext(r.Context(),
				`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at) VALUES (?, 'pluggy', ?, ?, ?)`,
				sourceID, itemID, now, now)
		}
		if err != nil {
			writeProblem(w, 503, "sync-run-unavailable", "Sincronização temporariamente indisponível", "Tente novamente em instantes.")
			return
		}

		runID := uuid.NewString()
		startedAt := db.FormatTime(time.Now())
		_, err = conn.ExecContext(r.Context(),
			`INSERT INTO sync_runs (id, source_id, status, started_at) VALUES (?, ?, 'in_progress', ?)`,
			runID, sourceID, startedAt)
		if err != nil {
			var active sql.NullString
			_ = conn.QueryRowContext(r.Context(),
				`SELECT id FROM sync_runs WHERE source_id = ? AND status = 'in_progress'`, sourceID,
			).Scan(&active)
			body := map[string]any{
				"type": "/problems/sync-already-running", "title": "Synchronization already in progress",
				"status": 409, "detail": "Wait for the active synchronization to finish.",
			}
			if active.Valid {
				body["active_sync_run_id"] = active.String
			}
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(409)
			_ = json.NewEncoder(w).Encode(body)
			return
		}

		row := conn.QueryRowContext(r.Context(), `
			SELECT id, status, started_at, finished_at, accounts_processed, transactions_inserted, transactions_updated
			FROM sync_runs WHERE id = ?`, runID)
		run, err := scanSyncRun(row)
		if err != nil {
			writeProblem(w, 503, "sync-run-unavailable", "Sincronização temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		w.Header().Set("Location", "/api/sync-runs/"+runID)
		writeJSON(w, http.StatusAccepted, run)
	}
}

func handleListSyncRuns(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := parsePositiveInt(v); err == nil && parsed >= 1 && parsed <= 100 {
				limit = parsed
			}
		}
		rows, err := conn.QueryContext(r.Context(), `
			SELECT id, status, started_at, finished_at, accounts_processed, transactions_inserted, transactions_updated
			FROM sync_runs ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
		if err != nil {
			writeProblem(w, 503, "sync-run-unavailable", "Sincronização temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		defer rows.Close()

		var result []syncRunDTO
		for rows.Next() {
			var (
				d             syncRunDTO
				startedAtRaw  string
				finishedAtRaw sql.NullString
			)
			if err := rows.Scan(&d.ID, &d.Status, &startedAtRaw, &finishedAtRaw,
				&d.AccountsProcessed, &d.TransactionsInserted, &d.TransactionsUpdated); err != nil {
				writeProblem(w, 503, "sync-run-unavailable", "Sincronização temporariamente indisponível", "Tente novamente em instantes.")
				return
			}
			if d.StartedAt, err = db.ParseTime(startedAtRaw); err != nil {
				continue
			}
			if d.FinishedAt, err = db.ParseNullTime(finishedAtRaw); err != nil {
				continue
			}
			d.ResultMessage = resultMessages[d.Status]
			result = append(result, d)
		}
		if result == nil {
			result = []syncRunDTO{}
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errBadInt
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

var errBadInt = errors.New("not a positive integer")

type syncRunDetailDTO struct {
	syncRunDTO
	Failures []syncFailureDTO `json:"failures"`
}

func handleGetSyncRun(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		row := conn.QueryRowContext(r.Context(), `
			SELECT id, status, started_at, finished_at, accounts_processed, transactions_inserted, transactions_updated
			FROM sync_runs WHERE id = ?`, id)
		run, err := scanSyncRun(row)
		if errors.Is(err, sql.ErrNoRows) {
			writeProblem(w, 404, "sync-run-not-found", "Synchronization not found", "")
			return
		}
		if err != nil {
			writeProblem(w, 503, "sync-run-unavailable", "Sincronização temporariamente indisponível", "Tente novamente em instantes.")
			return
		}

		rows, err := conn.QueryContext(r.Context(), `
			SELECT stage, error_code, safe_message, external_account_id, external_transaction_id, created_at
			FROM sync_failures WHERE sync_run_id = ? ORDER BY created_at`, id)
		if err != nil {
			writeProblem(w, 503, "sync-run-unavailable", "Sincronização temporariamente indisponível", "Tente novamente em instantes.")
			return
		}
		defer rows.Close()

		failures := []syncFailureDTO{}
		for rows.Next() {
			var f syncFailureDTO
			var occurredAtRaw string
			if err := rows.Scan(&f.Stage, &f.Code, &f.Message, &f.ExternalAccountID, &f.ExternalTransactionID, &occurredAtRaw); err != nil {
				continue
			}
			if f.OccurredAt, err = db.ParseTime(occurredAtRaw); err != nil {
				continue
			}
			failures = append(failures, f)
		}
		writeJSON(w, http.StatusOK, syncRunDetailDTO{syncRunDTO: run, Failures: failures})
	}
}
