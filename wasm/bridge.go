//go:build js && wasm

package main

import (
	"bytes"
	"sync"
	"syscall/js"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MinReadBuffer is a blocking io.Reader backed by a bytes.Buffer.
// BubbleTea expects a blocking reader — returning (0, nil) causes
// the input parser to misbehave. This spin-waits when empty.
type MinReadBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

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

func (b *MinReadBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

var (
	fromJS = &MinReadBuffer{}
	fromGo = &struct {
		mu  sync.Mutex
		buf bytes.Buffer
	}{}
)

// RegisterBridge registers the JS global functions for WASM I/O.
// Follows the BigJk/bubbletea-in-wasm pattern:
// input is push, output is polled, resize is injected via prog.Send().
func RegisterBridge(prog *tea.Program) {
	global := js.Global()

	global.Set("bubbletea_write", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 {
			return nil
		}
		fromJS.Write([]byte(args[0].String()))
		return nil
	}))

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

	global.Set("bubbletea_resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return nil
		}
		prog.Send(tea.WindowSizeMsg{
			Width:  args[0].Int(),
			Height: args[1].Int(),
		})
		return nil
	}))
}
