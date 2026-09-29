package statementimport

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestFlashTemplateIsDetectedAndParsedWithoutErrors(t *testing.T) {
	filename, content, ok := Template(Format)
	if !ok {
		t.Fatal("Template(flash_csv) ok = false")
	}
	if filename != "modelo-extrato-flash.csv" {
		t.Fatalf("filename = %q", filename)
	}
	if !bytes.HasPrefix(content, []byte("\xef\xbb\xbf"+strings.Join(header, ",")+"\n")) {
		t.Fatalf("content does not start with BOM and header: %q", content)
	}
	if bytes.Contains(content, []byte("\r")) {
		t.Fatal("template must use LF line endings")
	}
	if !(flashAdapter{}).Detect(content) {
		t.Fatal("Detect(template) = false")
	}

	p, err := Parse(content, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != Format || p.Institution != "Flash" || len(p.Rows) != 3 {
		t.Fatalf("parsed %+v", p)
	}
	if p.PeriodStart != "2026-01-01" || p.PeriodEnd != "2026-01-03" {
		t.Fatalf("period %s..%s", p.PeriodStart, p.PeriodEnd)
	}
	wantAmounts := []string{"-49.90", "-100.00", "1200.00"}
	wantBalances := []string{"1050.10", "1100.00", "1200.00"}
	for i, row := range p.Rows {
		if row.Status != "new" || len(row.Errors) != 0 || len(row.Warnings) != 0 {
			t.Errorf("row %d: status=%q errors=%v warnings=%v", i, row.Status, row.Errors, row.Warnings)
		}
		if row.Amount != wantAmounts[i] || row.Balance != wantBalances[i] {
			t.Errorf("row %d: amount=%s balance=%s, want %s / %s", i, row.Amount, row.Balance, wantAmounts[i], wantBalances[i])
		}
		if !strings.HasPrefix(row.Description, "Exemplo - ") {
			t.Errorf("row %d description %q is not clearly fictitious", i, row.Description)
		}
	}
}

// Importing the template must never produce a "continuidade do saldo" warning:
// sorted by time, each balance moves from the previous one by exactly the
// newer row's amount.
func TestFlashTemplateBalancesAreContinuous(t *testing.T) {
	_, content, _ := Template("")
	p, err := Parse(content, "")
	if err != nil {
		t.Fatal(err)
	}
	rows := append([]Row(nil), p.Rows...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].OccurredAt < rows[j].OccurredAt })
	for i := 1; i < len(rows); i++ {
		prev, next := rows[i-1], rows[i]
		if prev.OccurredAt == next.OccurredAt {
			t.Fatalf("rows %d and %d share a timestamp", i-1, i)
		}
		balanceNext := decimal.RequireFromString(next.Balance)
		balancePrev := decimal.RequireFromString(prev.Balance)
		amount := decimal.RequireFromString(next.Amount)
		if !balanceNext.Sub(balancePrev).Equal(amount) {
			t.Errorf("balance %s -> %s does not match amount %s", prev.Balance, next.Balance, next.Amount)
		}
	}
}

func TestTemplateDefaultsToFlashAndRejectsUnknownFormats(t *testing.T) {
	defaultName, defaultContent, ok := Template("")
	if !ok {
		t.Fatal("Template(\"\") ok = false")
	}
	name, content, ok := Template("flash_csv")
	if !ok || name != defaultName || !bytes.Equal(content, defaultContent) {
		t.Fatalf("Template(\"\") differs from Template(flash_csv): %q vs %q", defaultName, name)
	}
	if name, content, ok := Template("nope"); ok || name != "" || content != nil {
		t.Fatalf("Template(nope) = %q, %q, %v; want ok=false", name, content, ok)
	}
}

// Every registered adapter must ship a template its own Detect accepts, so a
// new institution cannot forget or break the download.
func TestEveryAdapterTemplateIsDetectedByItsAdapter(t *testing.T) {
	for _, a := range adapters {
		filename, content := a.Template()
		if filename == "" || len(content) == 0 {
			t.Errorf("%s: empty template", a.Format())
		}
		if !a.Detect(content) {
			t.Errorf("%s: Detect rejects its own template", a.Format())
		}
	}
}
