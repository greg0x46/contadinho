package datasources_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"contadinho-go/internal/datasources"
	"contadinho-go/internal/db"
)

func newConn(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

// Name is the fallback chain every list, header and sync row shows a
// connection by, so each rung is worth pinning: a never-synced connection has
// nothing but its item id, and a user's own label has to survive the next
// sync overwriting display_name with whatever the institution calls itself.
func TestNameFallsBackFromLabelToInstitutionToItemID(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source datasources.DataSource
		want   string
	}{
		{"label wins", datasources.DataSource{
			Label: strp("Conta pessoal"), DisplayName: strp("Banco X"), ExternalItemID: "item-1",
		}, "Conta pessoal"},
		{"institution when unlabelled", datasources.DataSource{
			DisplayName: strp("Banco X"), ExternalItemID: "item-1",
		}, "Banco X"},
		{"item id when never synced", datasources.DataSource{ExternalItemID: "item-1"}, "item-1"},
		{"an empty label is not a name", datasources.DataSource{
			Label: strp(""), DisplayName: strp("Banco X"), ExternalItemID: "item-1",
		}, "Banco X"},
		{"an empty institution is not a name either", datasources.DataSource{
			DisplayName: strp(""), ExternalItemID: "item-1",
		}, "item-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.source.Name(); got != tc.want {
				t.Errorf("Name() = %q, want %q", got, tc.want)
			}
		})
	}
}

// An empty label must reach the database as NULL, or Name would return "" and
// every list would show a connection with no name at all. Whitespace-only
// input is the HTTP layer's job to trim (see trimmedLabel); by the time it
// gets here, blank means empty.
func TestCreateStoresAnEmptyLabelAsNoLabel(t *testing.T) {
	conn := newConn(t)
	blank, err := datasources.Create(context.Background(), conn, datasources.ProviderPluggy, "item-1", strp(""))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if blank.Label != nil {
		t.Errorf("Label = %q, want nil so Name falls through", *blank.Label)
	}
	if blank.Name() != "item-1" {
		t.Errorf("Name() = %q, want the item id", blank.Name())
	}
}

// Re-adding the same item would give its accounts a second identity rather
// than a second connection, so it has to be a typed conflict the API can turn
// into a 409 rather than a generic failure.
func TestCreateRejectsAnItemAlreadyRegistered(t *testing.T) {
	conn := newConn(t)
	ctx := context.Background()
	if _, err := datasources.Create(ctx, conn, datasources.ProviderPluggy, "item-1", nil); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := datasources.Create(ctx, conn, datasources.ProviderPluggy, "item-1", nil); !errors.Is(err, datasources.ErrDuplicate) {
		t.Errorf("second Create err = %v, want ErrDuplicate", err)
	}
}

// nil means "leave as is" — that is what lets a rename not restate whether
// the connection is active, and a deactivation not clear the user's label.
func TestUpdateLeavesOmittedFieldsAlone(t *testing.T) {
	conn := newConn(t)
	ctx := context.Background()
	created, err := datasources.Create(ctx, conn, datasources.ProviderPluggy, "item-1", strp("Conta pessoal"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	deactivated, err := datasources.Update(ctx, conn, created.ID, nil, boolp(false))
	if err != nil {
		t.Fatalf("Update deactivate: %v", err)
	}
	if deactivated.IsActive {
		t.Error("IsActive = true, want the connection retired")
	}
	if deactivated.Label == nil || *deactivated.Label != "Conta pessoal" {
		t.Errorf("Label = %v, want the rename to have survived a deactivation", deactivated.Label)
	}

	renamed, err := datasources.Update(ctx, conn, created.ID, strp("Empresa"), nil)
	if err != nil {
		t.Fatalf("Update rename: %v", err)
	}
	if renamed.IsActive {
		t.Error("IsActive = true, want a rename to leave the connection retired")
	}
	if renamed.Label == nil || *renamed.Label != "Empresa" {
		t.Errorf("Label = %v, want Empresa", renamed.Label)
	}

	// An empty label clears it, which is how the UI offers "remove the
	// nickname" without a second endpoint.
	cleared, err := datasources.Update(ctx, conn, created.ID, strp(""), nil)
	if err != nil {
		t.Fatalf("Update clear label: %v", err)
	}
	if cleared.Label != nil {
		t.Errorf("Label = %q, want nil", *cleared.Label)
	}
}

func TestGetAndUpdateReportAnUnknownID(t *testing.T) {
	conn := newConn(t)
	ctx := context.Background()
	if _, err := datasources.Get(ctx, conn, "does-not-exist"); !errors.Is(err, datasources.ErrNotFound) {
		t.Errorf("Get err = %v, want ErrNotFound", err)
	}
	if _, err := datasources.Update(ctx, conn, "does-not-exist", strp("x"), nil); !errors.Is(err, datasources.ErrNotFound) {
		t.Errorf("Update err = %v, want ErrNotFound", err)
	}
}

// ListActive is what "sync everything" iterates, so a retired connection
// dropping out of it is the whole point of deactivation — while List keeps
// showing it, because its accounts and history are still readable.
func TestListActiveExcludesRetiredConnectionsButListKeepsThem(t *testing.T) {
	conn := newConn(t)
	ctx := context.Background()
	first, err := datasources.Create(ctx, conn, datasources.ProviderPluggy, "item-1", nil)
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	if _, err := datasources.Create(ctx, conn, datasources.ProviderPluggy, "item-2", nil); err != nil {
		t.Fatalf("Create second: %v", err)
	}
	if _, err := datasources.Update(ctx, conn, first.ID, nil, boolp(false)); err != nil {
		t.Fatalf("Update: %v", err)
	}

	all, err := datasources.List(ctx, conn)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List returned %d connections, want both — deactivating is not deleting", len(all))
	}
	active, err := datasources.ListActive(ctx, conn)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(active) != 1 || active[0].ExternalItemID != "item-2" {
		t.Errorf("ListActive = %+v, want only the still-active connection", active)
	}
}
