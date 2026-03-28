//go:build js && wasm

package console

import "errors"

func checkConsole(f File) error {
	return errors.New("console unavailable in js/wasm")
}

func newMaster(f File) (Console, error) {
	return nil, errors.New("console unavailable in js/wasm")
}
