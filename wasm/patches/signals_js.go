//go:build js && wasm

package tea

// listenForResize is a no-op on js/wasm — there is no SIGWINCH.
// Resize events are delivered via the bubbletea_resize bridge function.
func (p *Program) listenForResize(done chan struct{}) {
	close(done)
}
