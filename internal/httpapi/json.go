package httpapi

import (
	"encoding/json"
	"net/http"
)

// decodeStrict mirrors the reference API's StrictModel (extra="forbid"):
// unknown fields in the request body are rejected rather than silently
// ignored, and trailing data after the JSON value is an error too.
func decodeStrict(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != nil {
		return nil // io.EOF is the well-formed case: no trailing data.
	}
	return errTrailingData
}

var errTrailingData = jsonTrailingDataError{}

type jsonTrailingDataError struct{}

func (jsonTrailingDataError) Error() string { return "unexpected trailing data after JSON body" }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
