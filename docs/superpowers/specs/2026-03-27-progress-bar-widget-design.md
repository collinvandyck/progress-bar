# Progress Bar Widget — Design Spec

A general-purpose BubbleTea v2 progress bar widget designed as a standalone, importable Go library. Provides a deterministic progress bar, key/value data display, and freeform sections — all fed by a pull-based data provider interface.

## Goals

- **General-purpose**: usable across multiple CLI tools, especially admin tooling
- **Importable**: standalone Go module, clean API, easy to embed in any BubbleTea v2 app or use standalone
- **Compact**: targets 3-6 key/value pairs and ~1 freeform section — small terminal footprint
- **Flexible**: configurable layout, optional output capture, composable into larger TUIs
- **Shareable**: WASM build target for browser-based demos (phase 2)

## Architecture

### Approach: Single Model, Configured Sections

One `progressbar.Model` owns everything — the bar, key/value pairs, and freeform sections. The host provides data through a `DataProvider` interface that the widget polls on each tick.

Two API layers:

- **Low-level**: `progressbar.Model` — a standard BubbleTea v2 `tea.Model`, composable into larger apps
- **High-level**: `progressbar.Run()` — manages the full BubbleTea lifecycle, blocks until the caller is done. Hides BubbleTea entirely from the caller.

The widget is lifecycle-agnostic. It renders whatever the provider says. `current == total` means a full bar, not "shut down." The caller manages start/stop.

### Output Display

The widget does not own output rendering. It is the caller's (or the parent model's) responsibility to decide where program output goes — above the widget, below it, or nowhere. The `Run()` helper may handle output-above-widget rendering, but the widget model itself is output-agnostic.

## Core Types

```go
package progressbar

// DataProvider is the single interface a host implements to feed data to the widget.
// The widget reads from it on every tick cycle.
type DataProvider interface {
    // Progress returns the current count and total.
    Progress() (current, total int)

    // KeyValues returns labeled data pairs for the standard KV area.
    KeyValues() []KeyValue

    // Sections returns freeform named content blocks.
    Sections() []Section
}

// KeyValue is a labeled data point displayed in the standard KV area.
type KeyValue struct {
    Key   string
    Value string
}

// Section is a named freeform content block.
type Section struct {
    Title   string
    Content string // plain text, potentially multi-line
}
```

## Layout

Three configurable layouts. The caller chooses at creation time.

```go
type Layout int

const (
    LayoutBarTop    Layout = iota // bar → kvs → sections
    LayoutBarBottom               // kvs → sections → bar (default)
    LayoutCompact                 // bar+% inline → kvs → sections
)
```

Default: `LayoutBarBottom`.

Rendering logic is behind a clean internal boundary so layouts can be reworked or added without breaking the public API.

## Output Capture

The widget optionally reads from an `io.Reader` to capture program output. It buffers lines internally (ring buffer, configurable size). The widget does not render captured output — it just holds it. The `Run()` helper or a parent model can decide how to display buffered lines.

```go
type Options struct {
    Provider DataProvider
    Output   io.Reader // optional — widget reads lines from this
    Layout   Layout
    // additional fields as needed
}
```

The caller adapts to `io.Reader` however they want — `io.Pipe()`, `cmd.StdoutPipe()`, etc.

### Convenience Helpers

Default implementations to cover common patterns:

```go
// Capture returns an io.Reader and an io.Writer.
// Write to the Writer, widget reads from the Reader.
// Common case: redirect a logger, capture stdout, etc.
func Capture() (io.Reader, io.Writer)

// CaptureCmd wires a *exec.Cmd's stdout and stderr
// into a single io.Reader the widget can consume.
func CaptureCmd(cmd *exec.Cmd) io.Reader
```

`Capture()` wraps `io.Pipe()` with buffering so writers don't block on a slow render tick. `CaptureCmd()` sets `cmd.Stdout` and `cmd.Stderr`, merges them, returns the reader.

## High-Level API

```go
// Run starts a BubbleTea program with the progress widget, blocks until
// the caller cancels the context or the program exits.
func Run(ctx context.Context, opts Options) error
```

`Run()` is for admin tooling and simple CLIs that don't have their own BubbleTea app. It creates the model, starts the tea program, and tears down on context cancellation.

## Package Structure

```
progress-bar/
├── go.mod
├── go.sum
├── justfile
├── model.go                   # Model, Options, Layout, DataProvider interface
├── capture.go                 # Capture(), CaptureCmd() helpers
├── view.go                    # rendering logic (layout switching)
├── update.go                  # tea.Model implementation (tick, provider reads)
├── examples/
│   ├── basic/main.go          # simplest case — provider, no output capture
│   ├── writer/main.go         # logger writing to Capture() writer
│   ├── subprocess/main.go     # CaptureCmd() with a shelled-out process
│   └── composed/main.go       # embedding Model in a larger BubbleTea app
└── wasm/                      # phase 2
    ├── build.go               # build tags
    ├── main_js.go             # WASM entrypoint — hooks tea I/O to JS
    └── embed/
        ├── index.html         # minimal page with xterm.js terminal
        └── terminal.js        # xterm.js ↔ WASM stdin/stdout bridge
```

## Examples

Four runnable examples, each demonstrating a different usage pattern:

- **basic**: `go run ./examples/basic` — DataProvider with a progress bar and KVs, no output capture. Simplest possible usage.
- **writer**: `go run ./examples/writer` — uses `Capture()` to redirect a logger's output through the widget.
- **subprocess**: `go run ./examples/subprocess` — uses `CaptureCmd()` to wrap a shelled-out process with progress display.
- **composed**: `go run ./examples/composed` — embeds `progressbar.Model` in a larger BubbleTea app alongside other components.

## WASM Shareability (Phase 2)

Compile the widget to WASM for browser-based interactive demos.

- **Go → WASM**: `GOOS=js GOARCH=wasm go build -o widget.wasm ./wasm/`
- **Browser terminal**: xterm.js renders ANSI output, forwards keyboard input to WASM stdin
- **BubbleTea I/O bridge**: WASM entrypoint swaps `tea.WithInput`/`tea.WithOutput` to JS-provided streams
- **Artifact**: `widget.wasm` + `index.html` + `terminal.js` = self-contained static directory, hostable anywhere

The WASM layer is a separate entrypoint that imports the `progressbar` package. The core library has zero WASM awareness — no build tags leak into library code.

**Hosting:** GitHub Pages, deployed from a `just wasm` build. The WASM workflow should be solid and prioritized — it's the primary way to share demos.

Build target: `just wasm`

## Build Targets (justfile)

```just
# Run an example
example name:
    go run ./examples/{{name}}

# Build WASM artifact (phase 2)
wasm:
    GOOS=js GOARCH=wasm go build -o wasm/embed/widget.wasm ./wasm/

# Run tests
test:
    go test ./...
```

## Edge Cases

- **`Progress()` returns `(0, 0)`**: render an empty bar at 0%. No division by zero, no indeterminate state — the widget is deterministic only. If the provider hasn't started yet, that's a valid state.
- **`Run()` output rendering**: in phase 1, `Run()` does not render captured output. It starts the TUI and tears down on context cancellation. Output rendering above/below the widget is a follow-up concern.

## Design Principles

- **Interfaces over implementations**: `DataProvider` and `io.Reader` are the contracts. The widget doesn't care how data is produced.
- **Don't paint into corners**: internal rendering is behind clean boundaries. Layouts, tick rates, buffer sizes — all adjustable without API breaks.
- **Caller adapts, widget consumes**: the widget reads from standard interfaces. The caller is responsible for producing data in the right shape.
- **Phase things**: WASM is phase 2. Output rendering in `Run()` can evolve independently. The widget model stays focused.
