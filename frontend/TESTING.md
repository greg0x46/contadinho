# Frontend tests

`npm test` runs Vitest with the config in `vite.config.ts`. CI runs the full
suite with `npm test -- --run`.

## Projects

The suite is split into two Vitest projects that run in the same pool:

- `dom`: jsdom plus `src/test/setup.dom.ts` (jest-dom matchers,
  `matchMedia`/`ResizeObserver` stubs, Testing Library cleanup). Runs every
  `*.test.tsx` and the `.ts` tests listed in `domTsTests`.
- `node`: plain Node environment, no setup file. Runs every other
  `*.test.ts` (API contracts, transport, presentation helpers, pure math).
  Node already provides `fetch`, `Response`, `File`, `Blob`, `FormData` and
  `Headers`.

A new `.ts` test that touches `window`, `document`, storage or jest-dom
matchers fails in `node` with a `ReferenceError` (or an unknown matcher).
Add its path to `domTsTests` in `vite.config.ts`, or name it `.tsx` if it
renders components.

`restoreMocks`, `clearMocks`, `pool: "threads"`, `testTimeout` and
`maxWorkers` are set once at the root and inherited by both projects
(`extends: true`). Each test file still runs isolated (`isolate` defaults to
`true`).

Run a single project:

```sh
npx vitest --run --project node
npx vitest --run --project dom
```

## Worker count

`maxWorkers` is 2. Because the projects inherit it, the `--maxWorkers` CLI
flag does not override it; use `VITEST_MAX_WORKERS=<n> npm test -- --run`
to try another value.

With 3 or more workers, `src/components/investments/InvestmentPositionForm.test.tsx`
fails in most runs: under CPU contention its first `findByRole` exceeds
Testing Library's default 1 s wait while antd renders the drawer. With 2
workers it passed in every run below, and 2 already gives most of the gain.
Raising the count needs that test (and any other timing-sensitive one)
made robust first.

## Benchmark

Local machine, 8 CPUs, Node 24, Vitest 4.1.11, sequential runs. Time is
Vitest's reported `Duration`. Peak RSS was not recorded (no `time -v`
available in the agent environment). Every run had 54 files / 419 tests.

| Config | Run 1 | Run 2 | Run 3 | Median | Failures |
| --- | --- | --- | --- | --- | --- |
| Baseline: jsdom everywhere, 1 worker | 208.62 s | 207.99 s | 208.14 s | 208.14 s | 0/3 |
| Split, 1 worker | 200.12 s | 199.83 s | 199.38 s | 199.83 s | 0/3 |
| Split, 2 workers (chosen) | 105.88 s | 109.59 s | 109.77 s | 109.59 s | 0/3 |
| Split, 3 workers | 81.50 s | 84.62 s | 84.57 s | 84.57 s | 3/3 |
| Split, 4 workers | 71.66 s | 73.49 s | 75.25 s | 73.49 s | 2/3 |

The chosen config cuts the median by 47% (208.14 s to 109.59 s). The split
alone saves about 8 s of jsdom setup (Vitest's `environment` time drops from
18.7 s to 11.1 s); the rest comes from the second worker. The 2-worker
config also passed an extra run under concurrent lint/build load (117.53 s,
not counted in the table).

GitHub Actions numbers come from the PR's own CI runs (`frontend` job,
`npm test -- --run` on `ubuntu-24.04`, 4 vCPU); no separate benchmark
workflow was added.
