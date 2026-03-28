//go:build js && wasm

package tea

// initInput is a no-op on js/wasm — there is no TTY to configure.
// Input is supplied via tea.WithInput() from the WASM bridge.
func (p *Program) initInput() error {
	return nil
}

const suspendSupported = false

// suspendProcess is a no-op on js/wasm.
func suspendProcess() {}
