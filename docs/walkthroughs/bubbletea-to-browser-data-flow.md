# How Updates Flow from BubbleTea to the Browser

A walkthrough of the `progress-bar` project: a Go BubbleTea widget that compiles
to WASM and runs in a browser via xterm.js.

---

## Table of Contents

1. [The Big Picture](#the-big-picture)
2. [The Library: DataProvider, Model, and View](#the-library)
3. [The WASM Entrypoint: Wiring BubbleTea for the Browser](#the-wasm-entrypoint)
4. [The Bridge: Go and JS Talking Through a Shared Buffer](#the-bridge)
5. [The Web Frontend: xterm.js Polling and Rendering](#the-web-frontend)
6. [Why Polling Works (And Why It Does Not Fall Apart)](#why-polling-works)
7. [The Build Pipeline: Vendor Patching for WASM](#the-build-pipeline)
8. [GitHub Actions Deployment](#github-actions-deployment)

---

## The Big Picture

Here is the full data flow from your application data to pixels on screen:

```
+------------------+        +------------------+        +------------------+
|  DataProvider    |  tick  |   BubbleTea      |  View  |  standardRenderer|
|  .Progress()    |------->|   Model.Update() |------->|  (v0.25 v1)     |
|  .KeyValues()   | every  |   reads provider |        |  line-level diff |
|  .Sections()    | 100ms  |   caches state   |        |  ANSI escape seq |
+------------------+        +------------------+        +--------+---------+
                                                                 |
                                                          io.Writer
                                                          (outputWriter)
                                                                 |
                                                                 v
                                                        +--------+---------+
                                                        |  fromGo buffer   |
                                                        |  (bytes.Buffer   |
                                                        |   + sync.Mutex)  |
                                                        +--------+---------+
                                                                 |
                                                         bubbletea_read()
                                                         polled every 100ms
                                                                 |
                                                                 v
                                                        +--------+---------+
                                                        |  JavaScript      |
                                                        |  setInterval     |
                                                        +--------+---------+
                                                                 |
                                                          term.write(data)
                                                                 |
                                                                 v
                                                        +------------------+
                                                        |   xterm.js       |
                                                        |   terminal       |
                                                        |   (browser DOM)  |
                                                        +------------------+
```

The reverse path (user input) is simpler: xterm.js pushes keystrokes into
`fromJS` via `bubbletea_write()`, and BubbleTea reads from that buffer as its
stdin. Resize events skip the buffer entirely and inject a `WindowSizeMsg`
directly via `prog.Send()`.

Let us walk through each piece.

---

## The Library

Three files make up the widget: `progressbar.go`, `model.go`, and `view.go`.

### DataProvider -- the data contract

```go
type DataProvider interface {
    Progress() (current, total int)
    KeyValues() []KeyValue
    Sections() []Section
}
```

This is the only thing a consumer implements. The widget never knows *what* it
is tracking -- it just asks "how far along are you?" and "what should I display?"
every tick. Think of it like an interface for a car dashboard gauge: the gauge
does not care if it is measuring fuel or oil pressure, it just reads a number.

The `Options` struct ties a provider to the model and lets the caller pick a
layout:

```go
type Options struct {
    Provider DataProvider
    Output   io.Reader // optional -- widget reads lines from this
    Layout   Layout    // LayoutBarBottom (default), LayoutBarTop, LayoutCompact
}
```

### Model -- the BubbleTea core

The `Model` follows the standard BubbleTea pattern: Init, Update, View.

```go
const defaultTickInterval = 100 * time.Millisecond

type TickMsg time.Time
```

**Init** kicks off a repeating tick:

```go
func (m Model) Init() tea.Cmd {
    return doTick()
}
```

**Update** is where data flows in. On every `TickMsg`, the model pulls fresh
data from the provider and caches it:

```go
case TickMsg:
    if m.opts.Provider != nil {
        m.current, m.total = m.opts.Provider.Progress()
        m.kvs = m.opts.Provider.KeyValues()
        m.sections = m.opts.Provider.Sections()
    }
    return m, doTick()
```

This is a *pull* model: the widget asks the provider, the provider does not push.
The tick fires every 100ms, which is fast enough for smooth visual updates but
not so fast that it burns CPU.

**Common pitfall:** If your `DataProvider` methods do anything slow (network
calls, disk I/O), they will block the BubbleTea event loop. Keep them fast --
read from cached/atomic values, compute on a background goroutine.

The demo provider in `wasm/main.go` shows the right pattern: an `atomic.Int64`
that a background goroutine increments, with the `Progress()` method just doing
a lock-free `Load()`.

### View -- rendering to a string

`view.go` builds the output string. There is no color, no lipgloss -- just
unicode box-drawing and plain text. The bar looks like:

```
[████████████░░░░░░░░░░░░░░] 45% (90/200)
```

The rendering logic accounts for terminal width (from `WindowSizeMsg` or
defaulting to 80 columns) and assembles components in the order dictated by
the `Layout` option:

```go
switch m.opts.Layout {
case LayoutBarTop, LayoutCompact:
    parts = append(parts, bar)       // bar first
    // then kvs, then sections
default: // LayoutBarBottom
    // kvs first, then sections
    parts = append(parts, bar)       // bar last
}
```

The final output is just `strings.Join(parts, "\n")` -- a plain multi-line
string. BubbleTea's renderer takes it from here.

---

## The WASM Entrypoint

`wasm/main.go` is where the browser-specific wiring happens.

### The shared buffers

Two buffers sit between Go and JavaScript:

```go
var (
    fromJS = &MinReadBuffer{}   // JS --> Go (user keystrokes)
    fromGo = &struct {          // Go --> JS (rendered output)
        mu  sync.Mutex
        buf bytes.Buffer
    }{}
)
```

Think of these as two one-way pipes. `fromJS` carries input (keyboard) into
BubbleTea. `fromGo` carries rendered output out to the browser.

### The outputWriter

```go
type outputWriter struct{}

func (outputWriter) Write(p []byte) (int, error) {
    fromGo.mu.Lock()
    defer fromGo.mu.Unlock()
    return fromGo.buf.Write(p)
}
```

This is the io.Writer that BubbleTea's renderer writes to. Instead of going to
stdout (which does not exist in WASM), it appends to the `fromGo` buffer.
JavaScript will drain this buffer on a poll cycle.

### Wiring BubbleTea

```go
prog := tea.NewProgram(
    model,
    tea.WithInput(fromJS),        // read stdin from our blocking buffer
    tea.WithOutput(outputWriter{}), // write stdout to our shared buffer
    tea.WithAltScreen(),           // use alternate screen (full redraw mode)
)
```

Three key options:

- **WithInput(fromJS)**: Replaces os.Stdin with the `MinReadBuffer`. BubbleTea's
  input goroutine will call `Read()` on this, which blocks (spin-waits) when
  empty.
- **WithOutput(outputWriter{})**: Replaces os.Stdout. Every ANSI escape sequence
  and rendered frame goes here.
- **WithAltScreen()**: Tells BubbleTea to use the alternate screen buffer. This
  means full-screen redraws with cursor repositioning, which is exactly what
  xterm.js expects.

The program runs on a background goroutine, and `main()` blocks forever with
`select {}` to keep the WASM instance alive:

```go
go func() {
    prog.Run()
}()

select {}
```

### The demo provider

The entrypoint also includes a `demoProvider` that simulates work:

```go
provider := &demoProvider{total: 200, start: time.Now()}

go func() {
    for provider.current.Load() < int64(provider.total) {
        time.Sleep(50 * time.Millisecond)
        provider.current.Add(1)
    }
}()
```

A background goroutine increments the counter every 50ms. The provider methods
read via `atomic.Int64.Load()`, so there is no lock contention with the
BubbleTea tick. This is the pattern you want for real usage too.

---

## The Bridge: Go and JS Talking Through a Shared Buffer

`wasm/bridge.go` registers three global JavaScript functions via `syscall/js`.

### bubbletea_write (JS to Go -- input)

```go
global.Set("bubbletea_write", js.FuncOf(func(_ js.Value, args []js.Value) any {
    fromJS.Write([]byte(args[0].String()))
    return nil
}))
```

When xterm.js receives a keystroke, it calls `bubbletea_write("k")`. The bytes
land in `fromJS`, where BubbleTea's input goroutine is spin-waiting to read
them.

### bubbletea_read (Go to JS -- output)

```go
global.Set("bubbletea_read", js.FuncOf(func(_ js.Value, _ []js.Value) any {
    fromGo.mu.Lock()
    defer fromGo.mu.Unlock()
    if fromGo.buf.Len() == 0 {
        return ""
    }
    s := fromGo.buf.String()
    fromGo.buf.Reset()
    return s
}))
```

This is the drain function. JavaScript calls it every 100ms. It grabs whatever
has accumulated in the buffer, returns it as a string, and resets the buffer.
The lock ensures we do not read a half-written frame.

### bubbletea_resize (JS to Go -- terminal dimensions)

```go
global.Set("bubbletea_resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
    prog.Send(tea.WindowSizeMsg{
        Width:  args[0].Int(),
        Height: args[1].Int(),
    })
    return nil
}))
```

This one bypasses the buffer entirely. It injects a `WindowSizeMsg` directly
into BubbleTea's message queue via `prog.Send()`. This is necessary because
resize is not byte data -- it is a structured event that BubbleTea's `Update`
function handles to adjust the model's `width` field.

### MinReadBuffer -- the blocking reader

```go
func (b *MinReadBuffer) Read(p []byte) (int, error) {
    for {
        b.mu.Lock()
        if b.buf.Len() > 0 {
            n, err := b.buf.Read(p)
            b.mu.Unlock()
            return n, err
        }
        b.mu.Unlock()
        time.Sleep(100 * time.Millisecond)
    }
}
```

**This is a subtle but critical piece.** BubbleTea expects a blocking reader for
stdin. If `Read()` returns `(0, nil)`, the input parser enters a tight loop and
misbehaves. The `MinReadBuffer` spin-waits with a 100ms sleep when empty,
which keeps the goroutine alive without burning cycles.

Yes, spin-waiting is ugly. But in WASM's single-threaded-ish runtime, there is
no `select` on a file descriptor, no `epoll`, no signal to wake on. This is
the pragmatic solution.

---

## The Web Frontend

### index.html

The HTML is minimal: a loading spinner, an error div, and a terminal container.
It pulls in four scripts:

1. `wasm_exec.js` -- Go's standard WASM bootstrap (copied from GOROOT at build time)
2. `xterm.js` -- the terminal emulator
3. `xterm-addon-fit` -- auto-sizes the terminal to its container
4. `xterm-addon-webgl` -- GPU-accelerated rendering (with DOM fallback)
5. `terminal.js` -- the glue code

### terminal.js -- the orchestration

The script follows a clear five-step sequence:

**Step 1: Load and start the Go WASM module**

```js
var go = new Go();
var result = await WebAssembly.instantiateStreaming(
    fetch(wasmUrl), go.importObject
);
go.run(result.instance);
```

`go.run()` calls the Go `main()` function, which starts BubbleTea on a
background goroutine and then calls `RegisterBridge(prog)`, which sets the
three global functions.

**Step 2: Wait for the bridge**

```js
function bridgeReady() {
    return (
        typeof globalThis.bubbletea_write === "function" &&
        typeof globalThis.bubbletea_read === "function" &&
        typeof globalThis.bubbletea_resize === "function"
    );
}
```

There is a race: `go.run()` is async -- the Go code runs on the next
microtask. So the JS polls every 50ms (up to 100 attempts / 5 seconds) for the
bridge functions to appear. Once they do, it proceeds.

**Step 3: Create the xterm.js terminal**

```js
var term = new Terminal({ /* theme, font, etc. */ });
var fitAddon = new FitAddon.FitAddon();
term.loadAddon(fitAddon);
term.open(document.getElementById("terminal"));
```

It also tries to load the WebGL addon for performance, with a graceful fallback:

```js
try {
    var webglAddon = new WebglAddon.WebglAddon();
    term.loadAddon(webglAddon);
} catch (e) {
    console.warn("WebGL addon failed, falling back to DOM renderer:", e);
}
```

**Step 4: Send initial terminal size**

```js
bubbletea_resize(term.cols, term.rows);
```

This is important. BubbleTea needs to know the terminal dimensions before it
renders the first frame. Without this, the model's `width` would be 0 and it
would fall back to the 80-column default.

**Step 5: Wire up I/O**

```js
// Input: push
term.onData(function (data) {
    bubbletea_write(data);
});

// Output: poll
setInterval(function () {
    var data = bubbletea_read();
    if (data && data.length > 0) {
        term.write(data);
    }
}, 100);

// Resize: push
term.onResize(function (size) {
    bubbletea_resize(size.cols, size.rows);
});
```

Input and resize are event-driven (push). Output is polled. This asymmetry is
deliberate and it is the key design decision that makes the whole thing work.
More on that next.

---

## Why Polling Works (And Why It Does Not Fall Apart)

You might look at the 100ms polling interval and think "that is going to be
janky" or "we are going to miss frames." Here is why it actually works fine.

### BubbleTea v1's standardRenderer does line-level diffs

BubbleTea (v0.25, the v1-era renderer) does not blast the entire screen on
every frame. Its `standardRenderer` compares the previous frame to the current
frame *line by line* and only emits ANSI escape sequences for lines that
changed. The output looks something like:

```
\033[3;1H[████████████░░░░░░░░░░░░░░] 47% (94/200)\033[4;1H...
```

That is: "move cursor to row 3, col 1, write this line, move to row 4, write
that line." Each render cycle produces a small burst of these cursor-addressed
updates.

### Coalescing in the buffer

Between JS poll intervals, BubbleTea might render 1 frame or 0 frames (the
tick is also 100ms, so they roughly align). All the ANSI output accumulates in
the `fromGo` buffer. When JS calls `bubbletea_read()`, it gets the *entire*
accumulated string in one shot.

Here is the key insight: **cursor-addressed updates coalesce cleanly.** If two
ticks wrote to the buffer before JS polled, you get two sets of cursor-move +
line-overwrite sequences. xterm.js processes them sequentially and the screen
ends up correct -- the second set of writes simply overwrites the first. There
is no tearing, no partial state, because each write targets specific screen
coordinates.

```
  Time -->

  Go tick 1:  writes "\033[3;1H[███░░░░] 15%"  to fromGo
  Go tick 2:  writes "\033[3;1H[████░░░] 20%"  to fromGo
  JS poll:    reads both, passes to term.write()
  xterm.js:   processes sequentially, screen shows 20%
```

If JS happens to poll between the two ticks, the user sees 15% then 20% on
separate cycles. If it polls after both, the user sees 20% directly. Either
way, the result is correct. You just skip an intermediate visual state, which
for a progress bar is perfectly fine.

### The tradeoffs

- **Latency**: Up to ~100ms between a state change and it appearing on screen.
  For a progress bar, imperceptible. For a text editor, you would want something
  faster (but you would not be building a text editor this way).
- **CPU**: The spin-wait in `MinReadBuffer` and the JS `setInterval` are
  low-cost. The real work is in the rendering diff, which BubbleTea keeps cheap.
- **Simplicity**: No callbacks, no shared-memory gymnastics, no message ports.
  Just a mutex-guarded buffer and a poll loop. Hard to get wrong.

---

## The Build Pipeline: Vendor Patching for WASM

This is the gnarliest part of the project, and it is worth understanding why it
exists.

### The problem

BubbleTea and its dependency `containerd/console` assume they are running on a
real OS with TTYs, signals, and file descriptors. In `js/wasm`, none of that
exists. The code will not compile because:

1. BubbleTea's `tty.go` tries to open `/dev/tty`
2. BubbleTea's `signals.go` listens for `SIGWINCH`
3. `containerd/console` calls platform-specific ioctl-style functions

### The solution: vendor and patch

The `justfile` recipe tells the story:

```
wasm:
    go mod vendor
    cp wasm/patches/tty_js.go wasm/patches/signals_js.go \
       vendor/github.com/charmbracelet/bubbletea/
    cp wasm/patches/console_js.go \
       vendor/github.com/containerd/console/
    GOOS=js GOARCH=wasm go build -mod=vendor -ldflags="-s -w" \
       -o wasm/web/progressbar.wasm ./wasm/
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/web/
    rm -rf vendor/
```

Step by step:

1. **`go mod vendor`** -- pulls all dependencies into `vendor/`
2. **Copy patch files** -- drops build-tag-guarded stubs into the vendored source
3. **`GOOS=js GOARCH=wasm go build -mod=vendor`** -- compiles using the patched vendor tree
4. **Copy `wasm_exec.js`** -- the Go runtime's JS bootstrap, required by the browser
5. **`rm -rf vendor/`** -- clean up, the vendor dir was only needed for the build

### What the patches do

Each patch file has the build tag `//go:build js && wasm`, so they only compile
for the WASM target. They replace platform-specific functions with no-ops or
error stubs.

**`tty_js.go`** (patched into BubbleTea):
```go
func (p *Program) initInput() error  { return nil }
func (p *Program) restoreInput() error { return nil }
func openInputTTY() (*os.File, error) {
    return nil, errors.New("unavailable in js/wasm")
}
```

No TTY to open, no raw mode to set. We supply our own input via `WithInput()`.

**`signals_js.go`** (patched into BubbleTea):
```go
func (p *Program) listenForResize(done chan struct{}) {
    close(done)
}
```

No `SIGWINCH` in the browser. Resize comes through `bubbletea_resize()` instead.

**`console_js.go`** (patched into containerd/console):
```go
func checkConsole(f File) error {
    return errors.New("console unavailable in js/wasm")
}
```

There is no console device. BubbleTea gracefully handles this error because we
are providing our own I/O.

### Why not fork?

You could fork BubbleTea and add WASM support directly. But then you are
maintaining a fork of a fast-moving library. The vendor-patch approach means
you always use the upstream release and just overlay the three small stub files
at build time. When BubbleTea updates, you update your `go.mod` and the patches
almost certainly still apply (they target platform-specific files that rarely
change). Pragmatic and low-maintenance.

---

## GitHub Actions Deployment

`.github/workflows/deploy-pages.yml` automates the build-and-deploy cycle.

```yaml
on:
  push:
    branches: [main]
  workflow_dispatch:
```

Triggers on every push to main, or manually.

The build job mirrors the `justfile` exactly:

```yaml
- name: Build WASM
  run: |
    go mod vendor
    cp wasm/patches/tty_js.go wasm/patches/signals_js.go \
       vendor/github.com/charmbracelet/bubbletea/
    cp wasm/patches/console_js.go vendor/github.com/containerd/console/
    GOOS=js GOARCH=wasm go build -mod=vendor -ldflags="-s -w" \
       -o wasm/web/progressbar.wasm ./wasm/
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/web/
```

Then it uploads `wasm/web/` as a Pages artifact and deploys it. The
`concurrency` block ensures only one deploy runs at a time, canceling
in-progress deploys if a new push lands.

One detail worth noting: the `-ldflags="-s -w"` flags strip debug info and
the symbol table from the WASM binary. WASM files are large (Go's runtime is
not small), so this helps. You are still looking at a multi-megabyte binary,
which is just the cost of doing Go-in-the-browser.

---

## Putting It All Together

Let us trace one complete cycle, from data changing to pixels updating.

```
1. Background goroutine:  provider.current.Add(1)         -- counter is now 95
2. BubbleTea tick fires:  TickMsg arrives in Update()
3. Update() calls:        provider.Progress() -> (95, 200)
                          provider.KeyValues() -> [...]
                          provider.Sections() -> [...]
4. BubbleTea calls:       View() -> renderView()
5. renderView() builds:   "[████████████░░░░░░░] 47% (95/200)\n..."
6. standardRenderer:      diffs against previous frame, emits ANSI escapes
7. ANSI bytes written to: outputWriter -> fromGo.buf
8. 100ms later, JS calls: bubbletea_read() -> gets the ANSI string
9. JS calls:              term.write(data)
10. xterm.js:             interprets ANSI escapes, updates the DOM/WebGL canvas
11. User sees:            the bar at 47%
```

The whole trip takes at most ~200ms in the worst case (100ms tick + 100ms poll
not aligned). In practice, the tick and poll intervals often overlap, and the
perceived latency is closer to 100ms. For a progress bar, that is butter.

---

## File Reference

| File | Purpose |
|------|---------|
| `progressbar.go` | DataProvider interface, Options, Layout enum |
| `model.go` | BubbleTea Model (Init/Update/View), tick loop |
| `view.go` | Rendering logic: bar, key-values, sections |
| `wasm/main.go` | WASM entrypoint, demo provider, BubbleTea setup |
| `wasm/bridge.go` | JS bridge: bubbletea_write/read/resize, MinReadBuffer |
| `wasm/web/terminal.js` | xterm.js setup, I/O wiring, poll loop |
| `wasm/web/index.html` | HTML shell, script loading |
| `wasm/patches/tty_js.go` | Stub: TTY functions for WASM |
| `wasm/patches/signals_js.go` | Stub: SIGWINCH listener for WASM |
| `wasm/patches/console_js.go` | Stub: containerd/console for WASM |
| `justfile` | Build recipes (wasm, serve, deploy) |
| `.github/workflows/deploy-pages.yml` | CI/CD to GitHub Pages |
