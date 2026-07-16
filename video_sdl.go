//go:build sdl && cgo

package main

import (
	"fmt"

	"github.com/veandco/go-sdl2/sdl"
)

func showVideoFrame(bus *Bus) error {
	data, ok := videoFrameBytes(bus)
	if !ok {
		return fmt.Errorf("VID_NSTART=0x%08X is outside RAM", bus.Asic.VIDNStart)
	}
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return err
	}
	defer sdl.Quit()
	window, err := sdl.CreateWindow("Moreno.WebTV", sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED, videoWidth, videoHeight, sdl.WINDOW_SHOWN)
	if err != nil {
		return err
	}
	defer window.Destroy()
	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		return err
	}
	defer renderer.Destroy()
	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_YUY2, sdl.TEXTUREACCESS_STREAMING, videoWidth, videoHeight)
	if err != nil {
		return err
	}
	defer texture.Destroy()
	if err := texture.Update(nil, data, videoWidth*2); err != nil {
		return err
	}
	if err := renderer.Copy(texture, nil, nil); err != nil {
		return err
	}
	renderer.Present()
	for {
		event := sdl.WaitEvent()
		switch event.(type) {
		case *sdl.QuitEvent:
			return nil
		case *sdl.KeyboardEvent:
			key := event.(*sdl.KeyboardEvent)
			if key.Type == sdl.KEYDOWN && key.Keysym.Sym == sdl.K_ESCAPE {
				return nil
			}
		}
	}
}
