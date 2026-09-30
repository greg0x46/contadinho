package httpapi

import (
	"net/http"
	"strconv"

	"contadinho-go/internal/statementimport"
)

// handleStatementTemplate serves the example CSV for a statement format so the
// import screen can offer it as a download. The content lives in
// statementimport, next to the parser, so the header cannot drift from it.
func handleStatementTemplate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filename, content, ok := statementimport.Template(r.URL.Query().Get("format"))
		if !ok {
			writeProblem(w, 422, "unknown-format", "Formato desconhecido", "Não há modelo para o formato informado.")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filename))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}
}
