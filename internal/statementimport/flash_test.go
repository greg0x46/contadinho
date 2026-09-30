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

const flashTestRow = "25/09/2026,12:06,Restaurante,\"-R$ 49,90\",Cartão,\"R$ 708,01\""

// Detect has to accept every header shape parseFlash accepts, otherwise a real
// export fails as "formato não reconhecido" before the parser ever sees it.
func TestFlashDetectMatchesParserHeaderRules(t *testing.T) {
	plain := "Data,Hora,Movimentação,Valor,Meio de Pagamento,Saldo"
	for _, tc := range []struct {
		name string
		data string
	}{
		{"plain LF", plain + "\n" + flashTestRow + "\n"},
		{"CRLF", plain + "\r\n" + flashTestRow + "\r\n"},
		{"BOM", "\ufeff" + plain + "\n" + flashTestRow + "\n"},
		{"BOM and CRLF", "\ufeff" + plain + "\r\n" + flashTestRow + "\r\n"},
		{"spaces after commas", "Data, Hora, Movimentação, Valor, Meio de Pagamento, Saldo\n" + flashTestRow + "\n"},
		{"spaces around fields", " Data , Hora , Movimentação , Valor , Meio de Pagamento , Saldo \n" + flashTestRow + "\n"},
		{"quoted header", "\"Data\",\"Hora\",\"Movimentação\",\"Valor\",\"Meio de Pagamento\",\"Saldo\"\n" + flashTestRow + "\n"},
		{"quoted header with spaces inside quotes", "\"Data \",\" Hora\",\"Movimentação\",\"Valor\",\"Meio de Pagamento\",\"Saldo\"\r\n" + flashTestRow + "\r\n"},
		{"header only", plain + "\n"},
		{"header without trailing newline", plain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !(flashAdapter{}).Detect([]byte(tc.data)) {
				t.Fatal("Detect = false, want true")
			}
		})
	}
}

func TestFlashDetectRejectsOtherHeaders(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"only a newline", "\n"},
		{"short header", "Data,Hora,Movimentação,Valor,Meio de Pagamento\n" + flashTestRow + "\n"},
		{"extra column", "Data,Hora,Movimentação,Valor,Meio de Pagamento,Saldo,Extra\n" + flashTestRow + "\n"},
		{"wrong column name", "Data,Hora,Descrição,Valor,Meio de Pagamento,Saldo\n" + flashTestRow + "\n"},
		{"different case", "data,hora,movimentação,valor,meio de pagamento,saldo\n" + flashTestRow + "\n"},
		{"columns out of order", "Hora,Data,Movimentação,Valor,Meio de Pagamento,Saldo\n" + flashTestRow + "\n"},
		{"semicolon separated", "Data;Hora;Movimentação;Valor;Meio de Pagamento;Saldo\n"},
		{"unterminated quote", "\"Data,Hora,Movimentação,Valor,Meio de Pagamento,Saldo\n" + flashTestRow + "\n"},
		{"data row instead of header", flashTestRow + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if (flashAdapter{}).Detect([]byte(tc.data)) {
				t.Fatal("Detect = true, want false")
			}
			if _, err := Parse([]byte(tc.data), ""); err == nil || err.Error() != "formato de arquivo não reconhecido" {
				t.Fatalf("Parse err = %v, want formato de arquivo não reconhecido", err)
			}
		})
	}
}

// Detect must only look at the first record: a malformed body is the parser's
// business, and reporting it there gives the user the specific error instead
// of a generic "não reconhecido".
func TestFlashDetectIgnoresTheBodyOfTheFile(t *testing.T) {
	data := []byte("Data,Hora,Movimentação,Valor,Meio de Pagamento,Saldo\n25/09/2026,12:06,\"Descrição sem fechar aspas,\"-R$ 49,90\",Cartão\n")
	if !(flashAdapter{}).Detect(data) {
		t.Fatal("Detect = false, want true regardless of a malformed body")
	}
	if _, err := Parse(data, ""); err == nil || err.Error() == "formato de arquivo não reconhecido" {
		t.Fatalf("Parse err = %v, want the parser's own error about the body", err)
	}
}

// Whatever header variant Detect accepts must also parse, so the two cannot
// disagree about a file again.
func TestFlashParseAcceptsDetectedHeaderVariants(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		eol    string
		bom    string
	}{
		{"spaces after commas", "Data, Hora, Movimentação, Valor, Meio de Pagamento, Saldo", "\n", ""},
		{"quoted", "\"Data\",\"Hora\",\"Movimentação\",\"Valor\",\"Meio de Pagamento\",\"Saldo\"", "\n", ""},
		{"CRLF", "Data,Hora,Movimentação,Valor,Meio de Pagamento,Saldo", "\r\n", ""},
		{"BOM with spaces and CRLF", "Data, Hora, Movimentação, Valor, Meio de Pagamento, Saldo", "\r\n", "\ufeff"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.bom + tc.header + tc.eol + flashTestRow + tc.eol)
			p, err := Parse(data, "")
			if err != nil {
				t.Fatal(err)
			}
			if p.Format != Format || p.Institution != "Flash" || len(p.Rows) != 1 || p.Rows[0].Status != "new" {
				t.Fatalf("parsed %+v", p)
			}
			if p.Rows[0].Amount != "-49.90" || p.Rows[0].Balance != "708.01" {
				t.Fatalf("amounts %+v", p.Rows[0])
			}
		})
	}
}

func TestFlashParseWithRequestedFormat(t *testing.T) {
	data := []byte("Data, Hora, Movimentação, Valor, Meio de Pagamento, Saldo\r\n" + flashTestRow + "\r\n")
	p, err := Parse(data, "flash_csv")
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != "flash_csv" || p.Institution != "Flash" || len(p.Rows) != 1 {
		t.Fatalf("parsed %+v", p)
	}

	// A format that no adapter implements never matches, even for a file that
	// would be detected without the request.
	if _, err := Parse(data, "outro_banco_csv"); err == nil || err.Error() != "formato solicitado não corresponde ao arquivo" {
		t.Fatalf("mismatching requested format err = %v, want formato solicitado não corresponde ao arquivo", err)
	}
	// The right format requested for a file that is not in it.
	other := []byte("Data,Valor\n25/09/2026,10\n")
	if _, err := Parse(other, "flash_csv"); err == nil || err.Error() != "formato solicitado não corresponde ao arquivo" {
		t.Fatalf("requested flash_csv for another file err = %v, want formato solicitado não corresponde ao arquivo", err)
	}
	if _, err := Parse(other, ""); err == nil || err.Error() != "formato de arquivo não reconhecido" {
		t.Fatalf("unrequested unknown file err = %v, want formato de arquivo não reconhecido", err)
	}
}
