# Live Components Lab

A Go workbench for the Live Components public DOM API. The dashboard follows the
provided design, with measured metrics, a throughput chart, SSE output, a component
preview, and session results. go-api-kit serves the dashboard, JSON endpoints,
assets, and loopback HTTP workload. The application is separate from the framework
core.

The examples module uses the sibling `../go-api-kit` checkout and requires Go
1.25.3 or newer. The root `go.work` connects the modules; the core library retains
its existing dependencies and Go version.

From the repository root:

```sh
go run ./examples/stress
```

Open **http://127.0.0.1:9120**. To choose another interface or port:

```sh
go run ./examples/stress -addr 127.0.0.1:9200
```

The server prints its URL. Ctrl+C cancels the workload, closes event streams, and
shuts down HTTP gracefully. Assets and templates are embedded; no Node.js, package
installation, frontend build, database, or external services are needed.

## Use the Lab

Select a suite, components, iterations, and requested workers, then choose **Run
Test**. **Stop** cancels preparation or execution. Only one benchmark runs at a
time. **All test suites** runs the six suites sequentially; a failure or cancellation
stops the queue. Empty cards and an empty results table mean no test has run.

Start with 100 components, 10 iterations, and 10 workers. Increase one setting at a
time. Large rendering/selector tests operate on the entire composed tree on each
work item, so their cost grows faster than the component count alone suggests.

| Suite | Measured unit and correctness checks |
| --- | --- |
| DOM Mutation | A mixed transaction with 19 DOM calls and one render, or a selected individual DOM operation. Checks structure, detachment, text, attributes, classes, and parsed output. |
| Component Composition | A nested composition. Verifies parent relationships, removal, clone independence, and rendered structure. |
| Concurrent Requests | An actual loopback HTTP request through a go-api-kit route. Independent requests clone immutable sources, mutate, insert/remove, render, and verify output. Shared-read requests only render a common immutable source. |
| Memory Endurance | A creation/mutation/render/disposal cycle. Replaces each instance repeatedly and samples process allocations, GC, and post-cleanup heap. |
| Selector Performance | A batch of four queries across the composed dataset. Checks matching identities, counts, and document order. |
| Large Render | One serialization of an unchanged prepared tree. Verifies the parsed component count, final component, and text outside the render timer. |

**Workload options** exposes individual DOM methods, small/medium/wide/deep/mutated
rendering shapes, HTTP ownership scenarios, and a per-suite timeout. Composition
uses a 4-level tree normally or 64 levels for the deep variant. Composed rendering
and selector datasets cap nesting at 256. Each component is permanently assigned
to one worker; effective concurrency is `min(requested workers, components)`.
Requested worker counts of 1, 10, 100, 500, and 1,000 are supported.

The preview uses the actual banner, card, and table templates. Set text, toggle an
attribute or class, insert/remove children, clone, reset, and inspect the resulting
HTML. Every change is rendered on the server through the public API. Clone IDs
are removed to avoid duplicates, and request text is passed to `SetTextContent`.

## Read measurements correctly

- Throughput counts **verified workload units per elapsed running second**. A
  transaction, query batch, render, cycle, and HTTP request are different units.
  DOM calls, renders, and requests are also reported separately. Setup is excluded;
  worker scheduling, metric recording, and correctness checks affect throughput.
- Individual DOM and render latency timers enclose the selected operation only.
  Mixed transaction timing includes its DOM calls and serialization, with final
  correctness checks outside that timer. Composition times construction and
  serialization; endurance times the whole cycle, including its verification.
  HTTP latency includes the real loopback request and body read, excluding the
  client's final DOM check. HTTP render p95 uses the server's serialization timer.
- Average, p50, p95, and p99 use monotonic durations. Percentiles are upper bounds
  from a fixed 2,048-bucket histogram, with at most 3.125% bucket width. Samples do
  not accumulate in an unbounded slice. Render p95 is separate from workload p95.
- Heap, allocations, allocated bytes, goroutines, and GC describe the **Go process**.
  Allocation deltas per successful unit are process estimates; telemetry, dashboard
  requests, and other goroutines contribute. They are not isolated allocation
  profiles. The conventional Go benchmarks below offer another measurement path.
- Components and DOM nodes describe the actual prepared source dataset. Node
  counts come from parsing its serialized HTML, including text nodes. Transient
  clones and HTTP connections are excluded from this dataset size.
- Endurance discards its worker closures and samples heap after an explicit GC.
  Retained heap capacity or one post-GC sample does not prove a leak. Compare
  repeated runs and use profiles before making a retention claim.
- A result passes only if all configured work items completed and their assertions
  passed. Failed and cancelled runs remain visible with an error in the JSON export.

Result rows are frozen at completion; the top heap card and goroutine count stay
live. Results are kept in memory, capped at 60 entries. Logs
are capped at 120 server entries and 160 displayed entries; charts retain 240
half-second samples. Short runs show a measured completion average when they end
before the first chart tick. Use the download button for detailed JSON results.

## Bounds and concurrency limitations

The runner validates 1–10,000 components, 1–10,000 iterations, at most 2,000,000
work items per suite, 1–1,000 requested workers, and 1–120 seconds per suite.
A 512 MiB Go heap guard is sampled during execution, not a hard operating-system
memory limit. Rendering and selector preparation also reject datasets whose node
count times effective workers exceeds 1,000,000, before starting workers. This
prevents large trees from being parsed simultaneously by excessive worker counts.
The dashboard permits at most 32 simultaneous SSE streams. Runs
remain cancellable, and external-origin start/stop requests are rejected.

The lab uses `v8.NewHandlerWithMiddleware` with an explicit empty stack. The kit's
default timeout buffers responses and cannot carry SSE, and request rate limits
would distort the HTTP workload. The existing stream limits, per-write deadlines,
workload timeouts, local binding, and shutdown cancellation remain in effect.
The HTTP workload also uses an explicit empty kit middleware stack. Its HTML is
serialized once, timed, and sent with `h.HTML`; dashboard pages use `h.Render`.

**Shared mutable components are not synchronized by the framework.** The normal
Lab rejects that scenario instead of masking it with a global mutex. Independent
ownership and immutable shared reads are tested separately. A passing race run
only covers the exercised paths, not arbitrary application sharing.

An isolated, deliberately unsafe test demonstrates the existing limitation:

```sh
go test -race -tags=unsafe_shared ./examples/stress -run '^TestUnsafeSharedMutation$' -count=1
```

This command is **expected to fail** with a race in concurrent `SetAttribute`
calls. It was run during implementation and did report that race. The build tag
excludes it from normal tests. It does not run in the dashboard or alter the
framework's concurrency architecture.

## Verification

From the repository root:

```sh
go fmt ./... ./examples/...
go vet ./... ./examples/...
go test ./... ./examples/...
go test -race ./... ./examples/...
go build ./... ./examples/...
go test ./examples/stress -run '^$' -bench . -benchmem -benchtime=300ms
```

The mixed Go benchmark includes correctness verification; the render Go benchmark
validates the prepared tree and final output outside its timed serialization loop.
Use repeated measurements on a quiet machine for comparisons. Browser checks
exercise real start/stop, configuration errors, live events, preview mutations,
result export, and desktop/mobile layouts. They are development verification,
not a runtime dependency.

The HTTP surface is intentionally small: `GET /`, `/health`, `/static/`, `/api/state`,
`/api/results`, `/api/events`, `/api/preview`; `POST /api/run` and `/api/stop`.
The dashboard and preview use Live Components. SSE updates the surrounding browser
interface with vanilla JavaScript; this example does not claim the framework has
client synchronization or browser event support.
