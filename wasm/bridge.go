//go:build js && wasm

package main

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"
)

// wasmInput is an io.Reader backed by a buffer that JS writes into via
// the bubbletea_write global function.
type wasmInput struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *wasmInput) Read(p []byte) (int, error) {
	for {
		w.mu.Lock()
		n, err := w.buf.Read(p)
		w.mu.Unlock()
		if n > 0 || err != io.EOF {
			return n, err
		}
		// Buffer empty — poll.
		time.Sleep(50 * time.Millisecond)
	}
}

func (w *wasmInput) Write(data []byte) {
	w.mu.Lock()
	w.buf.Write(data)
	w.mu.Unlock()
}

// wasmOutput is an io.Writer backed by a buffer that JS reads from via
// the bubbletea_read global function.
type wasmOutput struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *wasmOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *wasmOutput) Drain() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.buf.String()
	w.buf.Reset()
	return s
}

var (
	input  = &wasmInput{}
	output = &wasmOutput{}
)

// RegisterBridgeSimple registers JS global functions for the WASM demo.
// Uses an atomic width pointer instead of a tea.Program for resize handling,
// since we bypass BubbleTea's renderer for WASM (its cell-level ANSI diffs
// don't survive polled I/O).
func RegisterBridgeSimple(width *atomic.Int32) {
	global := js.Global()

	// bubbletea_write(string) — JS pushes keyboard input to Go.
	global.Set("bubbletea_write", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 {
			return nil
		}
		input.Write([]byte(args[0].String()))
		return nil
	}))

	// bubbletea_read() string — JS pulls rendered terminal output from Go.
	global.Set("bubbletea_read", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		return output.Drain()
	}))

	// bubbletea_resize(cols, rows) — JS tells Go about terminal size changes.
	global.Set("bubbletea_resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return nil
		}
		width.Store(int32(args[0].Int()))
		return nil
	}))
}
