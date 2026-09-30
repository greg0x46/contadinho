package statementimport

import "strings"

const flashTemplateFilename = "modelo-extrato-flash.csv"

// flashTemplateRows are invented example movements, newest first like a real
// export. Balances are coherent (each one is the previous balance plus the
// movement), so importing the template never trips the balance-continuity
// check.
var flashTemplateRows = []string{
	`03/01/2026,18:30,Exemplo - Restaurante,"-R$ 49,90",Cartão,"R$ 1.050,10"`,
	`02/01/2026,09:15,Exemplo - Mercado,"-R$ 100,00",Pagamento PIX,"R$ 1.100,00"`,
	`01/01/2026,08:00,Exemplo - Depósito,"R$ 1.200,00",Depósito,"R$ 1.200,00"`,
}

// flashTemplate builds the file from the same header the parser checks, so the
// two cannot drift apart. It starts with a UTF-8 BOM so spreadsheet software
// opens the accents correctly, which parseFlash and Detect both tolerate.
func flashTemplate() (string, []byte) {
	var b strings.Builder
	b.WriteString("\ufeff")
	b.WriteString(strings.Join(header, ","))
	b.WriteString("\n")
	for _, row := range flashTemplateRows {
		b.WriteString(row)
		b.WriteString("\n")
	}
	return flashTemplateFilename, []byte(b.String())
}
