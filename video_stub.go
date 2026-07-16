//go:build !sdl || !cgo

package main

import "fmt"

func showVideoFrame(*Bus) error {
	return fmt.Errorf("SDL video support requires CGO and the sdl build tag")
}
