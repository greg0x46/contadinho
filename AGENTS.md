# Julius: rules for contributors

These rules apply to every contributor, human or automated (Claude Code,
Codex, ...). `CLAUDE.md` imports this file, so all tools read the same rules.
Product context is in `README.md`.

Keep comments brief; don’t restate what the code already makes clear.

## Architecture

- One Go binary (`github.com/greg0x46/julius`, Go version in `go.mod`) that
  serves the HTTP API, runs the background workers and embeds the built React
  frontend.
- Database: SQLite by default (`modernc.org/sqlite`, no cgo), Postgres through
  `pgx` when the `-db` flag or `JULIUS_DB` holds a `postgres://` or
  `postgresql://` DSN. Migrations are embedded SQL applied by goose on open.
- `cmd/julius` wires everything (database, HTTP server, sync and quote
  workers) and has the `auth` and `seed` subcommands. `cmd/migrate-to-postgres`
  is a one-shot SQLite-to-Postgres copier.
- `internal/` holds the domain packages. `httpapi` is the `net/http` layer
  (stdlib routing, no framework): it decodes requests, calls domain packages
  and encodes responses and RFC 7807-style problems. `db` owns connections and
  migrations; `db/dbtest` is the shared test helper. The README's "Project
  structure" is a starting point, not an exhaustive list: look at `internal/`.
- Financial rules live in the backend domain packages. The frontend renders
  what the API returns instead of re-deriving it. Money is always an exact
  decimal (`shopspring/decimal` in Go, decimal strings in the frontend), never
  a float.
- Reuse the domain engines described in `.specs/motores-de-dominio.md` before
  inventing a new abstraction.
- Frontend (`frontend/`): React 19, TypeScript, Ant Design / Pro Components,
  TanStack Query, Vite, tested with Vitest and Testing Library. `src/api` has
  the HTTP client, contracts and query keys; `src/hooks`, `src/pages`,
  `src/components`, `src/presentation` and `src/theme` hold the rest.
  `frontend/go.mod` is intentional (it keeps Go tooling out of `node_modules`);
  do not delete it.
- Embedding: the `embedded_frontend` build tag embeds `internal/webui/dist`.
  Without the tag a small fallback page is used, so plain `go build` and
  `go test` never need a frontend build.
- Do not weaken the security model: private routes check the session in the
  backend, HTTP writes require an `Origin` equal to the configured public
  origin plus `X-Julius-Request: 1`, and secrets are encrypted at rest with a
  key that comes from the environment or a file, never from the repository,
  logs, arguments or `VITE_*` variables.

## Specs

`.specs/README.md` is the navigation hub; read it before touching a domain
area. The specs are written in Portuguese; keep that language when editing
them.

- `.specs/contextos/<context>/reference.md` is the living description of what
  each feature does today. Update the matching one in the same PR whenever
  behavior, routes, pages or packages of that context change. A stale
  `reference.md` is worse than none.
- `.specs/invariantes-financeiros.md` maps each core financial rule to its
  owning module and test. Change the matching row in the same PR as the rule.
  `TestMatrixReferencesExistingTests` (in `internal/invariants`) fails when the
  matrix cites a test that no longer exists, so update it when renaming tests.
- A problem found but out of scope is not fixed in passing: list it under
  "Divergências" in the invariants doc or report it in the PR.
- `.specs/motores-de-dominio.md` is the architecture overview; update it only
  when the domain engines themselves change.
- A design spec (`.specs/<feature>.md`) is only for a feature that needs design
  decisions recorded. When the feature ships, move what still matters into the
  context's `reference.md` and delete the design spec.
- Update the README "Features" section only for a notable user-visible feature.
- Migrations have their own notes in `internal/db/migrations/README.md`.

## Commands

Run everything from the repository root unless a `cd` is shown. Go must match
`go.mod`; Node must be 24 or newer. Run the checks for the part you changed;
CI runs all of them.

Backend:

```sh
go build ./...
go vet ./...
go test -p 1 -count=1 ./...                    # what CI runs
go test -count=1 ./internal/<pkg> -run <Name>  # one package or test
```

CI also runs `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`, which
needs network access. Format the Go files you change with `gofmt`, but do not
reformat unrelated files: CI does not check formatting and some existing files
are not gofmt-clean.

Frontend:

```sh
cd frontend
npm ci                      # install exactly what package-lock.json says
npm run lint
npm run typecheck
npm test -- --run           # always --run: plain `npm test` starts watch mode
npm run build               # writes frontend/dist (git-ignored)
```

- Run one project or file with `npx vitest --run --project node`,
  `npx vitest --run --project dom` or `npx vitest --run src/api/client.test.ts`.
- The suite has two Vitest projects. A new `.ts` test that needs the DOM must be
  added to `domTsTests` in `frontend/vite.config.ts` (or be a `.tsx` file);
  details in `frontend/TESTING.md`.
- `npm run test:e2e` (Playwright) has no config or tests yet; do not run it.
- CI also runs `npm audit --omit=dev --audit-level=high`.
- CI checks that the embed still builds: copy `frontend/dist` to
  `internal/webui/dist` and run
  `go build -tags embedded_frontend -o /dev/null ./cmd/julius`. Only needed when
  you change the embed or build wiring; `internal/webui/dist` is git-ignored.

Do not change `go.mod`, `go.sum`, `package.json` or `package-lock.json` unless
the task is a dependency change (Dependabot handles routine bumps).

## Database tests

- `go test` uses SQLite (temporary files) by default. Postgres tests skip
  themselves unless `JULIUS_TEST_POSTGRES_DSN` is set.
- For SQL behavior that can differ between dialects, write the test with
  `dbtest.Each` from `internal/db/dbtest`: it runs the body on SQLite and, when
  the DSN is set, on Postgres in a private schema that is dropped afterwards.
- Some Postgres tests (mainly in `internal/db`) run
  `DROP SCHEMA public CASCADE` on the database behind the DSN, and packages
  share it, so the DSN must point at a disposable database and the tests must
  run with `-p 1`.
- Disposable Postgres, same image and credentials as the CI service (pick a free
  local port; the credentials are synthetic and already public in `ci.yml`):

```sh
docker run -d --rm --name julius-pgtest \
  -e POSTGRES_USER=julius_test -e POSTGRES_PASSWORD=synthetic-test-password \
  -e POSTGRES_DB=julius_test -p 127.0.0.1:<port>:5432 postgres:17
export JULIUS_TEST_POSTGRES_DSN='postgres://julius_test:synthetic-test-password@127.0.0.1:<port>/julius_test?sslmode=disable'
go test -p 1 -count=1 ./...
docker stop julius-pgtest
```

- Never point the DSN at a real or shared database, and never take it from a
  config file, `JULIUS_DB` or a running instance. If no disposable Postgres is
  available, run the SQLite suite and say in the PR that the Postgres pass was
  not run.

Migrations: there are two independent goose sequences,
`internal/db/migrations/sqlite` and `internal/db/migrations/postgres`, and the
same step has a different number in each. Match steps by file suffix, create
both files when the step applies to both, use the next number of each
directory, and give each file `-- +goose Up` and `-- +goose Down` sections.
Never renumber or rewrite existing migrations; add a new one. Read
`internal/db/migrations/README.md` first (SQLite table rebuilds and foreign
keys have pitfalls).

## Conventions

- Code, comments, identifiers, tests, commit messages and PR text are in
  English. `.specs/` documents and user-facing UI text are in Portuguese
  (pt-BR).
- Keep changes scoped to the task. No drive-by refactors or reformatting.
- Never commit build or local artifacts: the `julius` binary, `*.db*`,
  `.env.local`, `frontend/dist`, `internal/webui/dist`, `node_modules`.
- Never commit machine-specific values (hostnames, ports, local paths, database
  names, credentials). Configuration comes from environment variables.
- Tests use fakes, `httptest` servers and synthetic data. No real financial
  data, secrets or personal data in code, fixtures, logs or PR text.

## Commits and releases

Commits and PR titles follow Conventional Commits: `type(scope): subject`, for
example `fix(syncsvc): refuse cross-account merges`. Types in use: `feat`,
`fix`, `perf`, `refactor`, `docs`, `test`, `chore`, `ci`, `build`. The scope is
the package or area (`db`, `frontend`, `home`, ...).

Pull requests are squash-merged and the PR title becomes the commit subject on
`main`. On every push to `main`, CI runs `scripts/next-version.sh` over the
commits since the last tag:

- `feat` bumps the minor version, `fix` and `perf` bump the patch version.
- A breaking change (`type!:` or a `BREAKING CHANGE:` footer) bumps the major
  version (the minor while on `0.x`).
- Every other type, and any non-conventional commit, publishes nothing.

So merging a `feat`, `fix` or `perf` to `main` publishes a new GitHub release,
which must be treated as a production release.
Choose the type deliberately: use `feat`, `fix` or `perf` only for a change that
users can notice (behavior, API, data correctness, performance). Tests, specs,
internal refactors, tooling, CI and dependency bumps are `test`, `docs`,
`refactor`, `chore`, `ci` and `build`; a "fix" that only adds tests is `test`.
When unsure, pick the type that does not release and explain why in the PR.

Automated work must never use breaking-change markers (`!`, `BREAKING CHANGE:`).
If a change breaks compatibility (API contract, stored data, configuration
names), do not publish it; report it for a human decision.

Commit messages and PR descriptions carry no `Co-Authored-By` trailer and no
"Generated with ..." footer.

## Branches and pull requests

- Automated work uses the branch `agent/<issue>-<slug>`: the issue number and a
  short lowercase kebab-case slug, e.g. `agent/43-ingestion-identity-idempotency`.
- The PR base is always `main`. The title is the Conventional Commit subject.
- The body has `## Summary`, `## Behavior notes` (when there are any) and
  `## Test plan`, and ends with `Closes #N`. The test plan lists the exact
  commands run and their result, and says plainly what was skipped and why.
- Update the specs in the same PR (see "Specs"). Do not open a PR that fails the
  checks above.
- Stay inside the issue. Report unrelated problems in the PR or as a new issue
  instead of fixing them.

## Protected paths

Do not create, edit, delete or rename these. If the task cannot be done without
them, stop and report instead.

- `.github/` (workflows, including `ci.yml`, and Dependabot configuration)
- `.claude/` and `.agent/` (agent configuration)
- `scripts/next-version.sh` (release version logic)
- Environment, key and secret files: `.env*`, `*.key`, `*.pem`, and anything
  else that holds credentials

Never read, print, copy or commit the contents of key or credential files, or
of any real database file.

## Forbidden operations

- Never push to or merge into `main` or `stage`, and never create or delete tags
  or releases. Push only your own `agent/<issue>-<slug>` branch.
- Never edit CI, workflows or the release script, and never skip, disable or
  weaken tests, lint rules or checks to get a green result.
- Never run the application or a dev server: `./dev.sh`, `go run ./cmd/julius`
  (including its `auth` and `seed` subcommands), the built binary,
  `npm run dev`, `npm run preview`.
- Never run `./build.sh` (it runs `npm install`, rewrites `internal/webui/dist`
  and builds the binary). Use the commands in "Commands" instead.
- Never trigger a sync, a quote refresh or any other call to an external
  service, and never use real credentials: Pluggy, market-data providers, real
  databases, encryption keys. Network access is only for installing
  dependencies and for the vulnerability checks CI already runs.
- Never use a real or shared database. Only SQLite temporary files and a
  disposable Postgres (see "Database tests").

When a task needs a protected path, a forbidden operation or a breaking change,
or when the requirements are unclear or infeasible, stop and report what is
missing instead of improvising.
