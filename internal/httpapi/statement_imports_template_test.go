package httpapi_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStatementTemplateDownload(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, endpoint := range []string{
		"/api/statement-imports/template",
		"/api/statement-imports/template?format=flash_csv",
	} {
		resp := doJSON(t, http.MethodGet, srv.URL+endpoint, nil)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200: %s", endpoint, resp.StatusCode, body)
		}
		if got := resp.Header.Get("Content-Type"); got != "text/csv; charset=utf-8" {
			t.Errorf("GET %s Content-Type = %q", endpoint, got)
		}
		if got := resp.Header.Get("Content-Disposition"); got != `attachment; filename="modelo-extrato-flash.csv"` {
			t.Errorf("GET %s Content-Disposition = %q", endpoint, got)
		}
		wantStart := "\xef\xbb\xbfData,Hora,Movimentação,Valor,Meio de Pagamento,Saldo\n"
		if !bytes.HasPrefix(body, []byte(wantStart)) {
			t.Errorf("GET %s body does not start with BOM and header: %q", endpoint, body)
		}
		if lines := strings.Count(string(body), "\n"); lines != 4 {
			t.Errorf("GET %s body has %d lines, want header + 3 rows", endpoint, lines)
		}
	}
}

func TestStatementTemplateRejectsUnknownFormat(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodGet, srv.URL+"/api/statement-imports/template?format=nope", nil)
	var problem map[string]any
	decodeJSON(t, resp, &problem)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	if problem["type"] != "/problems/unknown-format" || problem["title"] != "Formato desconhecido" {
		t.Errorf("problem = %+v", problem)
	}
}

func TestStatementTemplateRequiresAuthentication(t *testing.T) {
	srv, _ := newLockedTestServer(t)
	resp := doJSON(t, http.MethodGet, srv.URL+"/api/statement-imports/template", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", resp.StatusCode)
	}
}
