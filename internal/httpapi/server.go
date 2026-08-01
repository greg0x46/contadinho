package httpapi

import (
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"contadinho-go/internal/debts"
	"contadinho-go/internal/settings"
)

// onIgnoredHook wires debts.UnlinkIfPresent into every inclusion-changing
// path this package exposes (manual PUT, automation's apply-to-new and
// apply-retroactively): whenever a transaction transitions to "ignored",
// any debt link it holds is dropped, matching the reference calling
// unlink_if_present from those same places.
var onIgnoredHook = debts.UnlinkIfPresent

// NewServer wires every route this phase implements (health, setup/unlock,
// categories, transactions) plus the embedded frontend with SPA fallback,
// mirroring backend/app/main.py's create_app + backend/app/web/spa.py.
// Routes for sync runs, automation rules, and debts join this mux in later
// phases. session holds the passphrase-derived encryption key in memory once
// setup/unlock succeeds — see package settings.
func NewServer(db *sql.DB, frontend fs.FS, session *settings.Session) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handleHealth(db))

	mux.HandleFunc("GET /api/setup/status", handleSetupStatus(db, session))
	mux.HandleFunc("POST /api/setup", handleSetup(db, session))
	mux.HandleFunc("POST /api/unlock", handleUnlock(db, session))

	mux.HandleFunc("GET /api/categories", handleListCategories(db))
	mux.HandleFunc("POST /api/categories", handleCreateCategory(db))
	mux.HandleFunc("PATCH /api/categories/{id}", handleUpdateCategory(db))

	mux.HandleFunc("POST /api/transactions/query", handleQueryTransactions(db))
	mux.HandleFunc("GET /api/transactions/spending-by-category", handleSpendingByCategory(db))
	mux.HandleFunc("PUT /api/transactions/{id}/inclusion", handleSetTransactionInclusion(db))
	mux.HandleFunc("PUT /api/transactions/{id}/category", handleSetTransactionCategory(db))

	mux.HandleFunc("POST /api/sync-runs", handleCreateSyncRun(db))
	mux.HandleFunc("GET /api/sync-runs", handleListSyncRuns(db))
	mux.HandleFunc("GET /api/sync-runs/{id}", handleGetSyncRun(db))

	mux.HandleFunc("GET /api/automation-rules/condition-options", handleListConditionOptions(db))
	mux.HandleFunc("GET /api/automation-rules", handleListAutomationRules(db))
	mux.HandleFunc("POST /api/automation-rules", handleCreateAutomationRule(db))
	mux.HandleFunc("PUT /api/automation-rules/{id}", handleUpdateAutomationRule(db))
	mux.HandleFunc("PATCH /api/automation-rules/{id}", handleSetAutomationRuleActive(db))
	mux.HandleFunc("DELETE /api/automation-rules/{id}", handleDeleteAutomationRule(db))

	mux.HandleFunc("GET /api/debts", handleListDebts(db))
	mux.HandleFunc("POST /api/debts", handleCreateDebt(db))
	mux.HandleFunc("GET /api/debts/total-owed", handleDebtTotalOwed(db))
	mux.HandleFunc("GET /api/debts/eligible-transactions", handleListEligibleTransactions(db))
	mux.HandleFunc("GET /api/debts/{id}", handleGetDebt(db))
	mux.HandleFunc("PUT /api/debts/{id}", handleUpdateDebt(db))
	mux.HandleFunc("DELETE /api/debts/{id}", handleDeleteDebt(db))
	mux.HandleFunc("POST /api/debts/{id}/links", handleCreateDebtLink(db))
	mux.HandleFunc("DELETE /api/debts/{id}/links/{linkId}", handleDeleteDebtLink(db))

	mux.Handle("/", spaHandler(frontend))

	return lockGate(session, mux)
}

// unlockExemptAPIPaths are the only /api/* routes lockGate lets through
// while the session is locked — the ones that exist specifically to ask
// "are we configured/unlocked?" or to become so. Every other /api/* route
// requires an unlocked session, closing the gap where the frontend's
// SetupGate decided what to *render* but the API underneath never actually
// enforced it: nothing stopped a direct request from reading or writing
// financial data while the app was locked or never configured.
var unlockExemptAPIPaths = map[string]bool{
	"/api/setup/status": true,
	"/api/setup":        true,
	"/api/unlock":       true,
}

// lockGate mirrors nothing in the Python reference (which had no lock state
// at all — credentials came from a .env file present from process start).
// It sits in front of the whole mux rather than as a per-route middleware so
// no future route can be added to NewServer and accidentally forget it.
func lockGate(session *settings.Session, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !unlockExemptAPIPaths[r.URL.Path] {
			if _, unlocked := session.Key(); !unlocked {
				writeProblem(w, http.StatusLocked, "locked",
					"Aplicação bloqueada",
					"Desbloqueie com a senha definida na configuração antes de continuar.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// spaHandler mirrors register_spa_host: it serves a file from the embedded
// build when the request path names one, falls back to index.html for any
// other path so client-side routing works, and returns a 404 JSON body
// (never the SPA shell) for anything under /api/ or /health that no other
// handler claimed.
func spaHandler(frontend fs.FS) http.Handler {
	fileServer := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath := strings.TrimPrefix(r.URL.Path, "/")
		if requestedPath == "health" || strings.HasPrefix(requestedPath, "api/") || strings.HasPrefix(requestedPath, "health/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Not Found"})
			return
		}

		if requestedPath != "" {
			if f, err := frontend.Open(requestedPath); err == nil {
				info, statErr := f.Stat()
				f.Close()
				if statErr == nil && !info.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			if path.Ext(requestedPath) != "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Not Found"})
				return
			}
		}

		// Client-side routes (e.g. /transactions) and the bare "/" both fall
		// through to index.html. Serving its bytes directly (rather than
		// rewriting the request path and delegating to fileServer) avoids
		// http.FileServer's built-in redirect-away-from-"index.html"
		// behavior, which would otherwise bounce every SPA route in a loop.
		index, err := fs.ReadFile(frontend, "index.html")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Not Found"})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
}
