package db

import "strings"

// InClause renders the body of an `IN (...)` and the positional binds that
// fill it: `?,?,?` plus ids as []any, for n=3. Every store package that
// loads a set of ids by one query needs both halves, and each had grown its
// own copy of the same two lines; returning them together is what keeps a
// caller from getting one right and the other wrong.
//
// It is also the only place that would have to change if the `?` convention
// this package rewrites for Postgres (see postgres.go) ever moved.
//
// An empty ids yields ("", nil): `IN ()` is not valid SQL on either driver,
// so a caller with nothing to look up must short-circuit before building
// the query rather than emit that. Every caller here already returns an
// empty result in that case, which is why this reports it as empty output
// rather than an error.
//
// The bind count is bounded by the driver — 32766 on SQLite, 65535 on
// pgx — so a set larger than that has to be chunked by the caller. No read
// path in this app comes near it today.
func InClause(ids []string) (string, []any) {
	if len(ids) == 0 {
		return "", nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}
