package httpapi_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
)

// registerConnection adds a Pluggy connection over HTTP and returns its id —
// the setup every sync-run test needs now that the item id lives in
// data_sources rather than in a single setting.
func registerConnection(t *testing.T, srv *httptest.Server, itemID string, label *string) string {
	t.Helper()
	body := map[string]any{"external_item_id": itemID, "label": label}
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/data-sources", body)
	if resp.StatusCode != 201 {
		resp.Body.Close()
		t.Fatalf("register connection %q: status = %d, want 201", itemID, resp.StatusCode)
	}
	var created map[string]any
	decodeJSON(t, resp, &created)
	return created["id"].(string)
}

func TestDataSourceRegistryOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodGet, srv.URL+"/api/data-sources", nil)
	var listed []map[string]any
	decodeJSON(t, resp, &listed)
	if len(listed) != 0 {
		t.Fatalf("fresh install lists %d connections, want 0", len(listed))
	}

	label := "Conta pessoal"
	id := registerConnection(t, srv, "item-1", &label)

	// The same item twice would give one bank's accounts two identities.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/data-sources",
		map[string]any{"external_item_id": "item-1"})
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("duplicate item status = %d, want 409", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/data-sources",
		map[string]any{"external_item_id": "   "})
	resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("blank item status = %d, want 422", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/data-sources", nil)
	decodeJSON(t, resp, &listed)
	if len(listed) != 1 {
		t.Fatalf("listed %d connections, want 1", len(listed))
	}
	if listed[0]["name"] != label || listed[0]["is_active"] != true {
		t.Errorf("listed connection = %+v", listed[0])
	}

	// Renaming must not disturb the active flag, and vice versa.
	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/data-sources/"+id,
		map[string]any{"is_active": false})
	var patched map[string]any
	decodeJSON(t, resp, &patched)
	if patched["is_active"] != false || patched["label"] != label {
		t.Errorf("after deactivating = %+v", patched)
	}

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/data-sources/"+id,
		map[string]any{"label": "Renomeada"})
	decodeJSON(t, resp, &patched)
	if patched["is_active"] != false || patched["name"] != "Renomeada" {
		t.Errorf("after renaming = %+v", patched)
	}

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/data-sources/does-not-exist",
		map[string]any{"label": "x"})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("unknown connection status = %d, want 404", resp.StatusCode)
	}
}

// TestSyncRunsCoverEveryActiveConnection is the whole point of the registry:
// one request refreshes every bank, a connection already syncing is stepped
// over rather than failing the others, and each run says which bank it is.
func TestSyncRunsCoverEveryActiveConnection(t *testing.T) {
	srv, conn := newTestServer(t)
	personal := "Pessoal"
	business := "Empresa"
	personalID := registerConnection(t, srv, "item-1", &personal)
	businessID := registerConnection(t, srv, "item-2", &business)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs", nil)
	if resp.StatusCode != 202 {
		t.Fatalf("create status = %d, want 202", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		t.Errorf("Location = %q, want none when several runs started", loc)
	}
	var createResp struct {
		Runs      []map[string]any `json:"runs"`
		Requested int              `json:"requested"`
	}
	decodeJSON(t, resp, &createResp)
	runs := createResp.Runs
	if len(runs) != 2 {
		t.Fatalf("started %d runs, want one per connection", len(runs))
	}
	if createResp.Requested != 2 {
		t.Errorf("requested = %d, want 2", createResp.Requested)
	}
	bySource := map[string]map[string]any{}
	for _, run := range runs {
		bySource[run["source_id"].(string)] = run
	}
	if bySource[personalID]["source_name"] != personal || bySource[businessID]["source_name"] != business {
		t.Errorf("runs did not name their connections: %+v", runs)
	}

	// Both are mid-sync now, so there is nothing left to start.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs", nil)
	var conflict map[string]any
	decodeJSON(t, resp, &conflict)
	if resp.StatusCode != 409 || conflict["active_sync_run_id"] == nil {
		t.Errorf("all-busy create: status=%d body=%+v", resp.StatusCode, conflict)
	}

	// Finish the business one; a fresh request must start it again and step
	// over the personal one still running, rather than conflicting.
	finishRun(t, conn, bySource[businessID]["id"].(string))

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs", nil)
	if resp.StatusCode != 202 {
		t.Fatalf("partial create status = %d, want 202", resp.StatusCode)
	}
	decodeJSON(t, resp, &createResp)
	runs = createResp.Runs
	if len(runs) != 1 || runs[0]["source_id"] != businessID {
		t.Errorf("partial create started %+v, want only the free connection", runs)
	}
	// requested still counts the busy connection: the caller needs to see
	// that fewer runs started than were targeted, not just what did start.
	if createResp.Requested != 2 {
		t.Errorf("requested = %d, want 2 (including the still-busy connection)", createResp.Requested)
	}

	// Targeting one connection explicitly.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs",
		map[string]any{"source_id": personalID})
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("explicit busy connection status = %d, want 409", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs",
		map[string]any{"source_id": "does-not-exist"})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("explicit unknown connection status = %d, want 404", resp.StatusCode)
	}
}

// TestSyncRunsSkipDeactivatedConnections covers the retirement path: a
// deactivated connection keeps its history but stops being refreshed.
func TestSyncRunsSkipDeactivatedConnections(t *testing.T) {
	srv, _ := newTestServer(t)
	activeID := registerConnection(t, srv, "item-1", nil)
	retiredID := registerConnection(t, srv, "item-2", nil)

	resp := doJSON(t, http.MethodPatch, srv.URL+"/api/data-sources/"+retiredID,
		map[string]any{"is_active": false})
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs", nil)
	var createResp struct {
		Runs      []map[string]any `json:"runs"`
		Requested int              `json:"requested"`
	}
	decodeJSON(t, resp, &createResp)
	runs := createResp.Runs
	if len(runs) != 1 || runs[0]["source_id"] != activeID {
		t.Fatalf("started %+v, want only the active connection", runs)
	}
	if createResp.Requested != 1 {
		t.Errorf("requested = %d, want 1 (deactivated connections aren't targeted)", createResp.Requested)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs",
		map[string]any{"source_id": retiredID})
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("explicit deactivated connection status = %d, want 409", resp.StatusCode)
	}
}

// finishRun closes a run out the way the worker would, so a test can ask for
// the next one without waiting on a real sync.
func finishRun(t *testing.T, conn *sql.DB, runID string) {
	t.Helper()
	if _, err := conn.Exec(
		`UPDATE sync_runs SET status = 'completed', finished_at = started_at WHERE id = ?`, runID,
	); err != nil {
		t.Fatalf("finish run: %v", err)
	}
}

// The Location header a 201 hands back has to resolve; before GET by id
// existed it pointed at a route the mux had no handler for.
func TestCreatedConnectionIsReadableAtItsLocation(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/data-sources",
		map[string]any{"external_item_id": "item-1", "label": "Conta pessoal"})
	if resp.StatusCode != 201 {
		resp.Body.Close()
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	var created map[string]any
	decodeJSON(t, resp, &created)
	if location == "" {
		t.Fatal("expected a Location header")
	}

	resp = doJSON(t, http.MethodGet, srv.URL+location, nil)
	if resp.StatusCode != 200 {
		resp.Body.Close()
		t.Fatalf("GET %s status = %d, want 200", location, resp.StatusCode)
	}
	var fetched map[string]any
	decodeJSON(t, resp, &fetched)
	if fetched["id"] != created["id"] || fetched["name"] != "Conta pessoal" {
		t.Errorf("fetched = %+v, want the connection just created", fetched)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/data-sources/11111111-1111-4111-8111-111111111111", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("unknown connection status = %d, want 404", resp.StatusCode)
	}
}
