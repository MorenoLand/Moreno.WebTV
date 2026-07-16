package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	sdlInitVideo           = 0x00000020
	sdlWindowShown         = 0x00000004
	sdlRendererAccelerated = 0x00000002
	sdlTextureStreaming    = 1
	sdlPixelFormatYUY2     = 0x32595559
	sdlQuitEvent           = 0x100
)

type sdl2 struct {
	init             func(uint32) int32
	quit             func()
	getError         func() *byte
	createWindow     func(*byte, int32, int32, int32, int32, uint32) unsafe.Pointer
	destroyWindow    func(unsafe.Pointer)
	createRenderer   func(unsafe.Pointer, int32, uint32) unsafe.Pointer
	destroyRenderer  func(unsafe.Pointer)
	createTexture    func(unsafe.Pointer, uint32, int32, int32, int32) unsafe.Pointer
	destroyTexture   func(unsafe.Pointer)
	updateTexture    func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, int32) int32
	renderClear      func(unsafe.Pointer) int32
	renderCopy       func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) int32
	renderPresent    func(unsafe.Pointer)
	waitEventTimeout func(unsafe.Pointer, int32) int32
}

func loadSDL2() (*sdl2, error) {
	lib, err := syscall.LoadLibrary(sdl2DLLPath())
	if err != nil {
		return nil, err
	}
	s := &sdl2{}
	purego.RegisterLibFunc(&s.init, uintptr(lib), "SDL_Init")
	purego.RegisterLibFunc(&s.quit, uintptr(lib), "SDL_Quit")
	purego.RegisterLibFunc(&s.getError, uintptr(lib), "SDL_GetError")
	purego.RegisterLibFunc(&s.createWindow, uintptr(lib), "SDL_CreateWindow")
	purego.RegisterLibFunc(&s.destroyWindow, uintptr(lib), "SDL_DestroyWindow")
	purego.RegisterLibFunc(&s.createRenderer, uintptr(lib), "SDL_CreateRenderer")
	purego.RegisterLibFunc(&s.destroyRenderer, uintptr(lib), "SDL_DestroyRenderer")
	purego.RegisterLibFunc(&s.createTexture, uintptr(lib), "SDL_CreateTexture")
	purego.RegisterLibFunc(&s.destroyTexture, uintptr(lib), "SDL_DestroyTexture")
	purego.RegisterLibFunc(&s.updateTexture, uintptr(lib), "SDL_UpdateTexture")
	purego.RegisterLibFunc(&s.renderClear, uintptr(lib), "SDL_RenderClear")
	purego.RegisterLibFunc(&s.renderCopy, uintptr(lib), "SDL_RenderCopy")
	purego.RegisterLibFunc(&s.renderPresent, uintptr(lib), "SDL_RenderPresent")
	purego.RegisterLibFunc(&s.waitEventTimeout, uintptr(lib), "SDL_WaitEventTimeout")
	return s, nil
}

func showVideoFrame(bus *Bus) error {
	data, ok := videoFrameBytes(bus)
	if !ok {
		return fmt.Errorf("VID_NSTART=0x%08X is outside RAM", bus.Asic.VIDNStart)
	}
	s, err := loadSDL2()
	if err != nil {
		return fmt.Errorf("load SDL2: %w", err)
	}
	if s.init(sdlInitVideo) != 0 {
		return fmt.Errorf("SDL_Init: %s", s.errorString())
	}
	defer s.quit()
	title := []byte("Moreno.WebTV\x00")
	window := s.createWindow(&title[0], 0x1FFF0000, 0x1FFF0000, videoWidth, videoHeight, sdlWindowShown)
	if window == nil {
		return fmt.Errorf("SDL_CreateWindow: %s", s.errorString())
	}
	defer s.destroyWindow(window)
	renderer := s.createRenderer(window, -1, sdlRendererAccelerated)
	if renderer == nil {
		return fmt.Errorf("SDL_CreateRenderer: %s", s.errorString())
	}
	defer s.destroyRenderer(renderer)
	texture := s.createTexture(renderer, sdlPixelFormatYUY2, sdlTextureStreaming, videoWidth, videoHeight)
	if texture == nil {
		return fmt.Errorf("SDL_CreateTexture: %s", s.errorString())
	}
	defer s.destroyTexture(texture)
	if s.updateTexture(texture, nil, unsafe.Pointer(unsafe.SliceData(data)), videoWidth*2) != 0 {
		return fmt.Errorf("SDL_UpdateTexture: %s", s.errorString())
	}
	if s.renderClear(renderer) != 0 || s.renderCopy(renderer, texture, nil, nil) != 0 {
		return fmt.Errorf("SDL_RenderCopy: %s", s.errorString())
	}
	s.renderPresent(renderer)
	event := [56]byte{}
	for {
		if s.waitEventTimeout(unsafe.Pointer(&event[0]), 16) != 0 && *(*uint32)(unsafe.Pointer(&event[0])) == sdlQuitEvent {
			return nil
		}
	}
}

func (s *sdl2) errorString() string {
	p := s.getError()
	if p == nil {
		return "unknown error"
	}
	n := 0
	for n < 1024 && *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
