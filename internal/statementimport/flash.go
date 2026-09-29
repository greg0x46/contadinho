package statementimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
)

const Format = "flash_csv"
const Version = "1"
const MaxBytes = 2 << 20
const MaxRows = 10000

var header = []string{"Data", "Hora", "Movimentação", "Valor", "Meio de Pagamento", "Saldo"}
var brl = regexp.MustCompile(`^-?R\$ ?(?:\d{1,3}(?:\.\d{3})*|\d+),\d{2}$`)

type Row struct {
	LineNumber    int      `json:"line_number"`
	OccurredAt    string   `json:"occurred_at,omitempty"`
	Description   string   `json:"description,omitempty"`
	Amount        string   `json:"amount,omitempty"`
	Currency      string   `json:"currency,omitempty"`
	PaymentMethod string   `json:"payment_method,omitempty"`
	Balance       string   `json:"balance,omitempty"`
	Status        string   `json:"status"`
	Errors        []string `json:"errors"`
	Warnings      []string `json:"warnings"`
	Identity      string   `json:"-"`
	ContentHash   string   `json:"-"`
}

type Parsed struct {
	Format        string   `json:"format"`
	FormatVersion string   `json:"format_version"`
	Institution   string   `json:"-"`
	SHA256        string   `json:"sha256"`
	Rows          []Row    `json:"rows"`
	PeriodStart   string   `json:"period_start,omitempty"`
	PeriodEnd     string   `json:"period_end,omitempty"`
	Currency      string   `json:"currency"`
	Warnings      []string `json:"warnings"`
}

func parseFlash(data []byte) (Parsed, error) {
	p := Parsed{Format: Format, FormatVersion: Version, Currency: "BRL", Rows: []Row{}, Warnings: []string{}}
	if len(data) == 0 || len(data) > MaxBytes {
		return p, errors.New("tamanho de arquivo inválido")
	}
	if !utf8.Valid(data) {
		return p, errors.New("arquivo deve estar em UTF-8")
	}
	sum := sha256.Sum256(data)
	p.SHA256 = hex.EncodeToString(sum[:])
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	reader.FieldsPerRecord = -1
	first, err := reader.Read()
	if err != nil {
		return p, errors.New("CSV inválido")
	}
	if len(first) != len(header) {
		return p, errors.New("cabeçalho Flash não reconhecido")
	}
	for i, v := range first {
		if strings.TrimSpace(v) != header[i] {
			return p, errors.New("cabeçalho Flash não reconhecido")
		}
	}
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	rownum := 1
	for {
		fields, e := reader.Read()
		if e == io.EOF {
			break
		}
		rownum++
		if rownum > MaxRows+1 {
			return p, errors.New("arquivo excede limite de linhas")
		}
		if e != nil {
			return p, errors.New("estrutura CSV inválida")
		}
		if len(fields) != 6 {
			return p, fmt.Errorf("linha %d com número incorreto de colunas", rownum)
		}
		row := Row{LineNumber: rownum, Description: strings.TrimSpace(fields[2]), PaymentMethod: strings.TrimSpace(fields[4]), Currency: "BRL", Status: "new", Errors: []string{}, Warnings: []string{}}
		stamp, er := time.ParseInLocation("02/01/2006 15:04", strings.TrimSpace(fields[0])+" "+strings.TrimSpace(fields[1]), loc)
		if er != nil || stamp.Format("02/01/2006 15:04") != strings.TrimSpace(fields[0])+" "+strings.TrimSpace(fields[1]) {
			row.Errors = append(row.Errors, "data ou hora inválida")
		} else {
			row.OccurredAt = db.FormatTime(stamp)
			day := stamp.Format("2006-01-02")
			if p.PeriodStart == "" || day < p.PeriodStart {
				p.PeriodStart = day
			}
			if day > p.PeriodEnd {
				p.PeriodEnd = day
			}
		}
		if row.Description == "" {
			row.Errors = append(row.Errors, "movimentação vazia")
		}
		if row.PaymentMethod == "" {
			row.Errors = append(row.Errors, "meio de pagamento vazio")
		}
		amount, er := parseMoney(fields[3])
		if er != nil {
			row.Errors = append(row.Errors, "valor inválido")
		} else if amount.IsZero() {
			row.Errors = append(row.Errors, "valor zerado")
		} else {
			row.Amount = amount.StringFixed(2)
		}
		balance, er := parseMoney(fields[5])
		if er != nil {
			row.Errors = append(row.Errors, "saldo inválido")
		} else {
			row.Balance = balance.StringFixed(2)
		}
		if len(row.Errors) > 0 {
			row.Status = "invalid"
		}
		p.Rows = append(p.Rows, row)
	}
	if len(p.Rows) == 0 {
		return p, errors.New("CSV sem movimentações")
	}
	return p, nil
}

func parseMoney(raw string) (decimal.Decimal, error) {
	s := strings.ReplaceAll(strings.TrimSpace(raw), "\u00a0", " ")
	s = strings.ReplaceAll(s, "\u202f", " ")
	if !brl.MatchString(s) {
		return decimal.Zero, errors.New("moeda inválida")
	}
	s = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, "R$", ""), " ", ""), ".", "")
	return decimal.NewFromString(strings.Replace(s, ",", ".", 1))
}
