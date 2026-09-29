package statementimport

import "testing"

func TestFlashParserSynthetic(t *testing.T) {
	data := []byte("\ufeffData,Hora,Movimentação,Valor,Meio de Pagamento,Saldo\n25/09/2026,12:06,Restaurante,\"-R$ 49,90\",Cartão,\"R$ 708,01\"\n08/09/2026,03:17,Depósito,\"R$ 1.533,33\",Depósito,\"R$ 1.533,33\"\n")
	p, e := Parse(data, "")
	if e != nil {
		t.Fatal(e)
	}
	if p.Format != Format || p.PeriodStart != "2026-09-08" || p.PeriodEnd != "2026-09-25" || len(p.Rows) != 2 {
		t.Fatalf("parsed %+v", p)
	}
	if p.Rows[0].Amount != "-49.90" || p.Rows[1].Amount != "1533.33" {
		t.Fatalf("amounts %+v", p.Rows)
	}
	if p.Rows[0].OccurredAt != "2026-09-25T15:06:00.000000000Z" {
		t.Fatalf("timezone %s", p.Rows[0].OccurredAt)
	}
}
func TestFlashParserInvalidRow(t *testing.T) {
	data := []byte("Data,Hora,Movimentação,Valor,Meio de Pagamento,Saldo\n31/02/2026,12:00,X,\"R$ 1,00\",PIX,\"R$ 1,00\"\n")
	p, e := Parse(data, Format)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Rows) != 1 || p.Rows[0].Status != "invalid" {
		t.Fatalf("row %+v", p.Rows)
	}
}
