package statementimport

import "errors"

// Adapter separates format recognition from normalization and storage.
// A new institution adds an adapter and registers it here.
type Adapter interface {
	Format() string
	Version() string
	Institution() string
	Detect([]byte) bool
	Parse([]byte) (Parsed, error)
	// Template is a small, fictitious example file in this adapter's format:
	// what a user downloads to see the layout Detect and Parse accept.
	Template() (filename string, content []byte)
}

type flashAdapter struct{}

func (flashAdapter) Format() string      { return Format }
func (flashAdapter) Version() string     { return Version }
func (flashAdapter) Institution() string { return "Flash" }

// Detect accepts exactly what parseFlash accepts as a header. It reads only the
// first CSV record, so the rest of the file is neither consumed nor validated
// here; any read error means "not this format".
func (flashAdapter) Detect(data []byte) bool {
	first, err := newFlashReader(data).Read()
	return err == nil && matchesFlashHeader(first)
}
func (flashAdapter) Parse(data []byte) (Parsed, error) { return parseFlash(data) }
func (flashAdapter) Template() (string, []byte)        { return flashTemplate() }

var adapters = []Adapter{flashAdapter{}}

// Template returns the example file for a format. An empty format means the
// default one, flash_csv; ok is false when no adapter implements the format.
func Template(format string) (filename string, content []byte, ok bool) {
	if format == "" {
		format = Format
	}
	for _, a := range adapters {
		if a.Format() == format {
			filename, content = a.Template()
			return filename, content, true
		}
	}
	return "", nil, false
}

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
