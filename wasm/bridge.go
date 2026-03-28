//go:build js && wasm

package main

import (
	"sync"
	"sync/atomic"
	"syscall/js"
)

// wasmInput is an io.Reader backed by a buffer that JS writes into.
// Not used in direct rendering mode but kept for future use.
type wasmInput struct {
	mu   sync.Mutex
	data []byte
}

func (w *wasmInput) Write(data []byte) {
	w.mu.Lock()
	w.data = append(w.data, data...)
	w.mu.Unlock()
}

var (
	input = &wasmInput{}
)

// RegisterBridgeSimple registers JS global functions for the WASM demo.
// The render function is called by JS on each animation frame — JS pulls
// frames instead of Go pushing them, which eliminates frame overlap.
func RegisterBridgeSimple(width *atomic.Int32, renderFn func() string) {
	global := js.Global()

	// bubbletea_write(string) — JS pushes keyboard input to Go.
	global.Set("bubbletea_write", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 {
			return nil
		}
		input.Write([]byte(args[0].String()))
		return nil
	}))

	// bubbletea_read() string — JS pulls the current rendered frame.
	// Called by JS on a setInterval. Each call renders a fresh frame
	// from the provider, so there's no buffer to accumulate or drain.
	global.Set("bubbletea_read", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		return renderFn()
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
