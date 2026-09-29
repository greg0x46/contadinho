package statementimport

import (
	"bytes"
	"errors"
)

// Adapter separates format recognition from normalization and storage.
// A new institution adds an adapter and registers it here.
type Adapter interface {
	Format() string
	Version() string
	Institution() string
	Detect([]byte) bool
	Parse([]byte) (Parsed, error)
}

type flashAdapter struct{}

func (flashAdapter) Format() string      { return Format }
func (flashAdapter) Version() string     { return Version }
func (flashAdapter) Institution() string { return "Flash" }
func (flashAdapter) Detect(data []byte) bool {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	first, _, _ := bytes.Cut(data, []byte{'\n'})
	return bytes.Equal(bytes.TrimSuffix(first, []byte{'\r'}), []byte("Data,Hora,Movimentação,Valor,Meio de Pagamento,Saldo"))
}
func (flashAdapter) Parse(data []byte) (Parsed, error) { return parseFlash(data) }

var adapters = []Adapter{flashAdapter{}}

func Parse(data []byte, requested string) (Parsed, error) {
	for _, a := range adapters {
		if requested != "" && requested != a.Format() {
			continue
		}
		if a.Detect(data) {
			parsed, err := a.Parse(data)
			parsed.Institution = a.Institution()
			return parsed, err
		}
	}
	if requested != "" {
		return Parsed{}, errors.New("formato solicitado não corresponde ao arquivo")
	}
	return Parsed{}, errors.New("formato de arquivo não reconhecido")
}
