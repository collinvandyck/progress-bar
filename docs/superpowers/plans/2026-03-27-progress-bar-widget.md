# Progress Bar Widget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a general-purpose BubbleTea v2 progress bar widget with a working WASM demo pipeline deployed to GitHub Pages.

**Architecture:** Single `progressbar.Model` fed by a pull-based `DataProvider` interface. Two API layers: low-level composable `tea.Model`, high-level `Run()` that hides BubbleTea. WASM pipeline compiles examples to browser-runnable artifacts via xterm.js.

**Tech Stack:** Go 1.24+, BubbleTea v2 (`charm.land/bubbletea/v2`), xterm.js 5.x, GitHub Pages, just

---

## File Structure

```
progress-bar/
├── go.mod                          # module github.com/collin/progress-bar
├── go.sum
├── justfile
├── .gitignore
├── progressbar.go                  # DataProvider, KeyValue, Section, Layout, Options — public types
├── progressbar_test.go             # tests for public types and helpers
├── model.go                        # Model struct, Init, Update (tea.Model impl)
├── model_test.go                   # model behavior tests
├── view.go                         # View() and layout rendering
├── view_test.go                    # view output tests
├── capture.go                      # Capture(), CaptureCmd()
├── capture_test.go                 # capture helper tests
├── run.go                          # Run() high-level API
├── examples/
│   ├── basic/main.go               # simplest usage — provider, no output capture
│   ├── writer/main.go              # Capture() with logger redirect
│   ├── subprocess/main.go          # CaptureCmd() wrapping exec.Cmd
│   └── composed/main.go            # embedding in a larger BubbleTea app
└── wasm/
    ├── bridge.go                   # WASM I/O bridge (syscall/js ↔ BubbleTea)
    ├── main.go                     # WASM entrypoint — demo using progressbar
    ├── platform_stubs_js.go        # build-tag stubs if needed for GOOS=js
    └── web/
        ├── index.html              # page with xterm.js terminal
        └── terminal.js             # xterm.js ↔ WASM global function wiring
```

---

### Task 1: Project Setup

**Files:**
- Create: `go.mod`, `.gitignore`, `justfile`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/collin/code/progress-bar
go mod init github.com/collin/progress-bar
```

- [ ] **Step 2: Add BubbleTea v2 dependency**

```bash
go get charm.land/bubbletea/v2@latest
```

- [ ] **Step 3: Create .gitignore**

```gitignore
# Build artifacts
*.wasm
wasm_exec.js

# IDE
.idea/
.vscode/

# OS
.DS_Store

# Superpowers brainstorm sessions
.superpowers/
```

- [ ] **Step 4: Create justfile**

```just
# Run an example
example name:
    go run ./examples/{{name}}

# Run tests
test:
    go test ./...

# Build WASM demo
wasm:
    GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o wasm/web/progressbar.wasm ./wasm/
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/web/

# Serve WASM demo locally
serve: wasm
    cd wasm/web && python3 -m http.server 8080

# Deploy to GitHub Pages (builds into docs/ for GH Pages)
deploy: wasm
    rm -rf docs/demo
    mkdir -p docs/demo
    cp wasm/web/* docs/demo/
```

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum .gitignore justfile
git commit -m "feat: initialize Go module with BubbleTea v2"
```

---

### Task 2: Core Public Types

**Files:**
- Create: `progressbar.go`, `progressbar_test.go`

- [ ] **Step 1: Write tests for core types**

```go
// progressbar_test.go
package progressbar

import "testing"

func TestKeyValueStringer(t *testing.T) {
	kv := KeyValue{Key: "Elapsed", Value: "1m 30s"}
	if kv.Key != "Elapsed" || kv.Value != "1m 30s" {
		t.Fatal("KeyValue fields not set correctly")
	}
}

func TestSectionContent(t *testing.T) {
	s := Section{Title: "Status", Content: "Processing batch 3 of 10"}
	if s.Title != "Status" || s.Content != "Processing batch 3 of 10" {
		t.Fatal("Section fields not set correctly")
	}
}

func TestLayoutDefaults(t *testing.T) {
	if LayoutBarBottom != 0 {
		t.Fatal("LayoutBarBottom should be 0 (iota, default)")
	}
	if LayoutBarTop != 1 {
		t.Fatal("LayoutBarTop should be 1")
	}
	if LayoutCompact != 2 {
		t.Fatal("LayoutCompact should be 2")
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{}
	if opts.Layout != LayoutBarBottom {
		t.Fatal("zero-value Layout should be LayoutBarBottom")
	}
	if opts.Provider != nil {
		t.Fatal("zero-value Provider should be nil")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./... -v
```

Expected: compilation failure (types don't exist yet)

- [ ] **Step 3: Implement core types**

```go
// progressbar.go
package progressbar

import "io"

// DataProvider is the interface a host implements to feed data to the widget.
// The widget reads from it on every tick cycle.
type DataProvider interface {
	// Progress returns the current count and total.
	Progress() (current, total int)

	// KeyValues returns labeled data pairs for the standard KV area.
	KeyValues() []KeyValue

	// Sections returns freeform named content blocks.
	Sections() []Section
}

// KeyValue is a labeled data point displayed in the KV area.
type KeyValue struct {
	Key   string
	Value string
}

// Section is a named freeform content block.
type Section struct {
	Title   string
	Content string
}

// Layout controls the arrangement of widget components.
type Layout int

const (
	LayoutBarBottom Layout = iota // kvs → sections → bar (default, zero value)
	LayoutBarTop                  // bar → kvs → sections
	LayoutCompact                 // bar+% inline → kvs → sections
)

// Options configures the progress bar widget.
type Options struct {
	Provider DataProvider
	Output   io.Reader // optional — widget reads lines from this
	Layout   Layout
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./... -v
```

Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add progressbar.go progressbar_test.go
git commit -m "feat: add core types — DataProvider, KeyValue, Section, Layout, Options"
```

---

### Task 3: Minimal Model (tea.Model Implementation)

**Files:**
- Create: `model.go`, `model_test.go`

- [ ] **Step 1: Write tests for model basics**

```go
// model_test.go
package progressbar

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stubProvider implements DataProvider for tests.
type stubProvider struct {
	current, total int
	kvs            []KeyValue
	sections       []Section
}

func (s *stubProvider) Progress() (int, int)  { return s.current, s.total }
func (s *stubProvider) KeyValues() []KeyValue  { return s.kvs }
func (s *stubProvider) Sections() []Section    { return s.sections }

func TestNewModel(t *testing.T) {
	p := &stubProvider{current: 0, total: 100}
	m := New(Options{Provider: p})
	if m.opts.Provider == nil {
		t.Fatal("provider should be set")
	}
}

func TestModelImplementsTeaModel(t *testing.T) {
	p := &stubProvider{current: 0, total: 100}
	m := New(Options{Provider: p})
	// Verify it satisfies tea.Model at compile time
	var _ tea.Model = m
}

func TestModelInit(t *testing.T) {
	p := &stubProvider{current: 0, total: 100}
	m := New(Options{Provider: p})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a tick command")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./... -v
```

Expected: compilation failure

- [ ] **Step 3: Implement Model**

```go
// model.go
package progressbar

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const defaultTickInterval = 100 * time.Millisecond

// TickMsg triggers a re-read from the DataProvider.
type TickMsg time.Time

// Model is a BubbleTea v2 model that renders a progress bar widget.
type Model struct {
	opts Options

	// Cached state from last provider read
	current  int
	total    int
	kvs      []KeyValue
	sections []Section

	width int
}

// New creates a new progress bar Model.
func New(opts Options) Model {
	return Model{opts: opts}
}

func doTick() tea.Cmd {
	return tea.Tick(defaultTickInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Init starts the tick loop.
func (m Model) Init() tea.Cmd {
	return doTick()
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		if m.opts.Provider != nil {
			m.current, m.total = m.opts.Provider.Progress()
			m.kvs = m.opts.Provider.KeyValues()
			m.sections = m.opts.Provider.Sections()
		}
		return m, doTick()
	case tea.WindowSizeMsg:
		wsm := msg.(tea.WindowSizeMsg)
		m.width = wsm.Width
		return m, nil
	}
	return m, nil
}

// View renders the widget.
func (m Model) View() tea.View {
	return tea.NewView(m.renderView())
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./... -v
```

Expected: compilation failure still — `renderView()` doesn't exist. That's Task 4.

- [ ] **Step 5: Add stub renderView to unblock compilation**

```go
// view.go
package progressbar

// renderView produces the string content for the widget.
func (m Model) renderView() string {
	return ""
}
```

- [ ] **Step 6: Run tests — expect pass**

```bash
go test ./... -v
```

Expected: all pass

- [ ] **Step 7: Commit**

```bash
git add model.go model_test.go view.go
git commit -m "feat: add Model with tea.Model implementation and tick loop"
```

---

### Task 4: View Rendering

**Files:**
- Modify: `view.go`
- Create: `view_test.go`

- [ ] **Step 1: Write view tests**

```go
// view_test.go
package progressbar

import (
	"strings"
	"testing"
)

func TestRenderProgressBar(t *testing.T) {
	m := Model{
		current: 50,
		total:   100,
		width:   40,
	}
	bar := m.renderBar()
	if !strings.Contains(bar, "50%") {
		t.Fatalf("expected bar to contain '50%%', got: %s", bar)
	}
}

func TestRenderBarZeroTotal(t *testing.T) {
	m := Model{
		current: 0,
		total:   0,
		width:   40,
	}
	bar := m.renderBar()
	if !strings.Contains(bar, "0%") {
		t.Fatalf("expected bar to contain '0%%', got: %s", bar)
	}
}

func TestRenderKeyValues(t *testing.T) {
	m := Model{
		kvs: []KeyValue{
			{Key: "Elapsed", Value: "1m 30s"},
			{Key: "Rate", Value: "500/s"},
		},
		width: 60,
	}
	out := m.renderKeyValues()
	if !strings.Contains(out, "Elapsed") || !strings.Contains(out, "1m 30s") {
		t.Fatalf("expected KVs in output, got: %s", out)
	}
	if !strings.Contains(out, "Rate") || !strings.Contains(out, "500/s") {
		t.Fatalf("expected KVs in output, got: %s", out)
	}
}

func TestRenderSections(t *testing.T) {
	m := Model{
		sections: []Section{
			{Title: "Status", Content: "Processing batch 3 of 10"},
		},
		width: 60,
	}
	out := m.renderSections()
	if !strings.Contains(out, "Status") || !strings.Contains(out, "Processing batch 3") {
		t.Fatalf("expected section in output, got: %s", out)
	}
}

func TestRenderViewLayoutBarBottom(t *testing.T) {
	m := Model{
		opts:    Options{Layout: LayoutBarBottom},
		current: 25,
		total:   100,
		kvs:     []KeyValue{{Key: "Elapsed", Value: "30s"}},
		width:   40,
	}
	out := m.renderView()
	barIdx := strings.Index(out, "25%")
	kvIdx := strings.Index(out, "Elapsed")
	if kvIdx > barIdx {
		t.Fatal("LayoutBarBottom: KVs should appear before bar")
	}
}

func TestRenderViewLayoutBarTop(t *testing.T) {
	m := Model{
		opts:    Options{Layout: LayoutBarTop},
		current: 25,
		total:   100,
		kvs:     []KeyValue{{Key: "Elapsed", Value: "30s"}},
		width:   40,
	}
	out := m.renderView()
	barIdx := strings.Index(out, "25%")
	kvIdx := strings.Index(out, "Elapsed")
	if barIdx > kvIdx {
		t.Fatal("LayoutBarTop: bar should appear before KVs")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./... -v
```

Expected: `renderBar`, `renderKeyValues`, `renderSections` don't exist

- [ ] **Step 3: Implement view rendering**

Replace the stub `view.go` with full rendering logic. Use `lipgloss` for styling if available, or plain string building to start. The bar uses Unicode block characters (`█` for filled, `░` for empty). KVs render as `Key: Value` separated by `  `. Sections render as `Title\nContent`. Layout switches on `m.opts.Layout` to order the components.

Key implementation details:
- `renderBar()` — computes fill width from `current/total * available width`, renders `████░░░░ XX% (current/total)`
- `renderKeyValues()` — joins KVs as `Key: Value` with spacing
- `renderSections()` — each section as `Title\nContent` with a separator
- `renderView()` — assembles components in layout order with newlines between them
- Guard against `total == 0` in percentage calculation

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./... -v
```

- [ ] **Step 5: Commit**

```bash
git add view.go view_test.go
git commit -m "feat: add view rendering with configurable layouts"
```

---

### Task 5: Basic Example

**Files:**
- Create: `examples/basic/main.go`

- [ ] **Step 1: Write the basic example**

A self-contained program that creates a `DataProvider` which simulates work (incrementing a counter on a goroutine), creates the widget, and runs it with `tea.NewProgram`. Exits on `q` or `ctrl+c`.

The example should:
- Implement `DataProvider` with a struct that has an atomic counter
- Increment the counter in a goroutine with `time.Sleep` between increments
- Show 3-4 key/value pairs (Elapsed, Rate, Items)
- Show one freeform section (Status)
- Use `LayoutBarBottom`

- [ ] **Step 2: Run it**

```bash
just example basic
```

Expected: a working progress bar in the terminal. Verify it renders, updates, and exits cleanly.

- [ ] **Step 3: Commit**

```bash
git add examples/basic/main.go
git commit -m "feat: add basic example"
```

---

### Task 6: WASM Bridge

**Files:**
- Create: `wasm/bridge.go`, `wasm/main.go`

This is the critical task. The bridge connects BubbleTea's I/O to JavaScript global functions that xterm.js calls.

- [ ] **Step 1: Create the WASM I/O bridge**

`wasm/bridge.go` (build tag `//go:build js && wasm`):

Implements:
- `wasmInput` — an `io.Reader` backed by a `bytes.Buffer`. JS writes keyboard input via `bubbletea_write` global. `Read()` polls the buffer with a short sleep when empty.
- `wasmOutput` — an `io.Writer` backed by a `bytes.Buffer`. Go writes terminal output here. JS reads via `bubbletea_read` global.
- `RegisterBridge(prog *tea.Program)` — registers three `syscall/js` global functions:
  - `bubbletea_write(string)` — JS → Go input
  - `bubbletea_read() string` — Go → JS output
  - `bubbletea_resize(cols, rows)` — sends `tea.WindowSizeMsg` to the program

- [ ] **Step 2: Create the WASM entrypoint**

`wasm/main.go` (build tag `//go:build js && wasm`):

- Creates a demo `DataProvider` that simulates work (same pattern as basic example, but using JS-friendly timing)
- Creates `progressbar.Model` with the provider
- Creates `tea.NewProgram` with `tea.WithInput(wasmInput)` and `tea.WithOutput(wasmOutput)`
- Calls `RegisterBridge(prog)`
- Runs the program
- Blocks with `select{}` after program exits

- [ ] **Step 3: Attempt WASM compilation**

```bash
GOOS=js GOARCH=wasm go build -o /dev/null ./wasm/
```

This is the moment of truth. BubbleTea v2 may or may not compile for `GOOS=js`. If it fails:
- Identify which files/functions fail (likely TTY and signal handling)
- Check if v2 already has `_js.go` build-tagged files
- If not, we may need to vendor or use build-tag overlay files

Document any compilation issues and fixes needed.

- [ ] **Step 4: Fix any GOOS=js compilation issues**

If BubbleTea v2 doesn't compile for `GOOS=js`, create `wasm/platform_stubs_js.go` or use `go.mod` replacements as needed. This may require:
- Filing an issue upstream
- Creating minimal stubs for missing platform implementations
- Using a `replace` directive in `go.mod` if vendoring is needed

The goal is: `GOOS=js GOARCH=wasm go build ./wasm/` succeeds.

- [ ] **Step 5: Commit**

```bash
git add wasm/
git commit -m "feat: add WASM bridge and entrypoint"
```

---

### Task 7: WASM Web Frontend

**Files:**
- Create: `wasm/web/index.html`, `wasm/web/terminal.js`

- [ ] **Step 1: Create index.html**

Minimal HTML page:
- Loads `wasm_exec.js` (copied by `just wasm`)
- Loads xterm.js 5.x and xterm-addon-fit from CDN
- Loads `terminal.js`
- Has a full-viewport `<div id="terminal">`
- Dark background, no chrome

- [ ] **Step 2: Create terminal.js**

JavaScript that:
- Instantiates `Go()` and loads `progressbar.wasm` via `WebAssembly.instantiateStreaming`
- Waits for the global bridge functions (`bubbletea_write`, `bubbletea_read`, `bubbletea_resize`) to be registered
- Creates xterm.js `Terminal` with `FitAddon`
- Wires `term.onData` → `bubbletea_write` (keyboard input)
- Polls `bubbletea_read` on interval (50-100ms) → `term.write` (display output)
- Wires `term.onResize` and `window.resize` → `bubbletea_resize`
- Sends initial terminal size on connect

- [ ] **Step 3: Build and test locally**

```bash
just serve
```

Open `http://localhost:8080` in a browser. Verify:
- WASM loads without console errors
- xterm.js renders
- Progress bar appears and updates
- No visible input lag (polling latency is acceptable)

- [ ] **Step 4: Commit**

```bash
git add wasm/web/
git commit -m "feat: add WASM web frontend with xterm.js"
```

---

### Task 8: GitHub Pages Deployment

**Files:**
- Modify: `justfile`
- Create: `.github/workflows/deploy-pages.yml` (optional — or use manual deploy)

- [ ] **Step 1: Build the deployable artifact**

```bash
just deploy
```

This copies the WASM build output into `docs/demo/`. Verify the directory contains:
- `index.html`
- `terminal.js`
- `wasm_exec.js`
- `progressbar.wasm`

- [ ] **Step 2: Test the docs/demo/ directory locally**

```bash
cd docs/demo && python3 -m http.server 8080
```

Open in browser, verify it works identically to `just serve`.

- [ ] **Step 3: Configure GitHub Pages**

Two options (pick one based on preference):

**Option A — deploy from `docs/` directory:**
- In GitHub repo settings → Pages → Source: "Deploy from a branch", branch `main`, folder `/docs`
- Commit `docs/demo/` to the repo

**Option B — GitHub Actions workflow:**
- Create `.github/workflows/deploy-pages.yml` that runs `just wasm`, copies to the right place, and deploys
- Keeps build artifacts out of the repo

- [ ] **Step 4: Commit deployment config**

```bash
git add .
git commit -m "feat: add GitHub Pages deployment"
```

- [ ] **Step 5: Push and verify**

```bash
git push origin main
```

Verify the demo is accessible at `https://<username>.github.io/progress-bar/demo/`

---

### Task 9: Capture Helpers

**Files:**
- Create: `capture.go`, `capture_test.go`

- [ ] **Step 1: Write tests for Capture()**

```go
// capture_test.go
package progressbar

import (
	"io"
	"testing"
)

func TestCaptureWriteRead(t *testing.T) {
	r, w := Capture()
	msg := "hello from logger"
	go func() {
		w.Write([]byte(msg))
	}()
	buf := make([]byte, 256)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != msg {
		t.Fatalf("expected %q, got %q", msg, string(buf[:n]))
	}
}

func TestCaptureImplementsInterfaces(t *testing.T) {
	r, w := Capture()
	var _ io.Reader = r
	var _ io.Writer = w
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./... -v -run TestCapture
```

- [ ] **Step 3: Implement Capture()**

```go
// capture.go
package progressbar

import "io"

// Capture returns a connected Reader/Writer pair.
// Write to the Writer (e.g., redirect a logger), the widget reads from the Reader.
func Capture() (io.Reader, io.Writer) {
	r, w := io.Pipe()
	return r, w
}
```

Note: start with `io.Pipe()`. If blocking becomes an issue in practice, swap internals to a buffered pipe later — the public API stays the same.

- [ ] **Step 4: Write tests for CaptureCmd()**

```go
func TestCaptureCmdSetsStdout(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	r, err := CaptureCmd(cmd)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if !strings.Contains(string(buf[:n]), "hello") {
		t.Fatalf("expected 'hello' in output, got %q", string(buf[:n]))
	}
}
```

- [ ] **Step 5: Implement CaptureCmd()**

```go
// CaptureCmd wires a *exec.Cmd's stdout and stderr into a single io.Reader.
// It starts the command. The reader will return io.EOF when the command exits.
func CaptureCmd(cmd *exec.Cmd) (io.Reader, error) {
	r, w := io.Pipe()
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Start(); err != nil {
		w.Close()
		return nil, err
	}
	go func() {
		cmd.Wait()
		w.Close()
	}()
	return r, nil
}
```

- [ ] **Step 6: Run all tests — expect pass**

```bash
go test ./... -v
```

- [ ] **Step 7: Commit**

```bash
git add capture.go capture_test.go
git commit -m "feat: add Capture() and CaptureCmd() helpers"
```

---

### Task 10: Run() High-Level API

**Files:**
- Create: `run.go`
- Modify: `model_test.go` (add Run test)

- [ ] **Step 1: Write test for Run()**

```go
func TestRunExitsOnContextCancel(t *testing.T) {
	p := &stubProvider{current: 100, total: 100}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{Provider: p})
	}()

	// Give it a moment to start
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./... -v -run TestRun
```

- [ ] **Step 3: Implement Run()**

```go
// run.go
package progressbar

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// Run starts a BubbleTea program with the progress widget.
// Blocks until ctx is cancelled or the program exits.
func Run(ctx context.Context, opts Options) error {
	m := New(opts)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./... -v
```

- [ ] **Step 5: Commit**

```bash
git add run.go model_test.go
git commit -m "feat: add Run() high-level API"
```

---

### Task 11: Remaining Examples

**Files:**
- Create: `examples/writer/main.go`, `examples/subprocess/main.go`, `examples/composed/main.go`

- [ ] **Step 1: Write writer example**

Demonstrates `Capture()` — creates a logger that writes to the capture writer, runs the widget with the reader attached, shows log output being captured while progress advances.

- [ ] **Step 2: Write subprocess example**

Demonstrates `CaptureCmd()` — runs a subprocess (e.g., `find / -name "*.go"` or a script that produces output), wraps it with progress display.

- [ ] **Step 3: Write composed example**

Demonstrates embedding `progressbar.Model` in a larger BubbleTea app — a parent model that contains the progress bar plus other UI elements (e.g., a title, a help line).

- [ ] **Step 4: Test each example**

```bash
just example writer
just example subprocess
just example composed
```

Verify each runs, renders correctly, and exits cleanly.

- [ ] **Step 5: Commit**

```bash
git add examples/
git commit -m "feat: add writer, subprocess, and composed examples"
```
