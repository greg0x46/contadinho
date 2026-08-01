// This file exists only to make frontend/ a separate Go module boundary, so
// `go build/vet/test ./...` at the repo root doesn't descend into
// node_modules (some npm packages, e.g. flatted, bundle a stray .go file)
// or treat this directory as part of the contadinho-go module at all — the
// frontend is a Vite/React project, not Go code.
module contadinho-go/frontend/_notgo

go 1.21
