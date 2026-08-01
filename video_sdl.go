package main

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"syscall"
	"time"
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
	videoStableDuration    = 150 * time.Millisecond
)

type stableVideoFrame struct {
	candidate []byte
	presented []byte
	since     time.Time
}

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

func showVideoFrame(bus *Bus, done <-chan struct{}) error {
	splash, err := bootSplashFrame(bus)
	if err != nil {
		return fmt.Errorf("boot splash: %w", err)
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
	started := time.Now()
	frames := stableVideoFrame{presented: splash}
	event := [56]byte{}
	for {
		data := frames.presented
		if time.Since(started) >= 4*time.Second {
			if frame, ok := videoFrameBytes(bus); ok {
				data = frames.update(frame, time.Now())
			}
		}
		if s.updateTexture(texture, nil, unsafe.Pointer(unsafe.SliceData(data)), videoWidth*2) != 0 {
			return fmt.Errorf("SDL_UpdateTexture: %s", s.errorString())
		}
		if s.renderClear(renderer) != 0 || s.renderCopy(renderer, texture, nil, nil) != 0 {
			return fmt.Errorf("SDL_RenderCopy: %s", s.errorString())
		}
		s.renderPresent(renderer)
		if s.waitEventTimeout(unsafe.Pointer(&event[0]), 16) != 0 && *(*uint32)(unsafe.Pointer(&event[0])) == sdlQuitEvent {
			return nil
		}
		select {
		case <-done:
			done = nil
		default:
		}
	}
}

func (f *stableVideoFrame) update(frame []byte, now time.Time) []byte {
	if !videoFramePresentable(frame) {
		f.candidate = f.candidate[:0]
		f.since = now
		return f.presented
	}
	if !bytes.Equal(frame, f.candidate) {
		f.candidate = append(f.candidate[:0], frame...)
		f.since = now
	} else if now.Sub(f.since) >= videoStableDuration && !bytes.Equal(f.presented, f.candidate) {
		f.presented = append(f.presented[:0], f.candidate...)
	}
	return f.presented
}

func videoFramePresentable(frame []byte) bool {
	magenta, samples := 0, 0
	for i := 0; i+3 < len(frame); i += 128 {
		u, v := frame[i+1], frame[i+3]
		if u > 170 && v > 170 {
			magenta++
		}
		samples++
	}
	return samples != 0 && magenta*2 < samples
}

func bootSplashFrame(bus *Bus) ([]byte, error) {
	end := 0xC00000 + int(bus.ROMSz)
	if end > len(bus.ROM) {
		return nil, fmt.Errorf("boot ROM size %d is invalid", bus.ROMSz)
	}
	data, err := readROMFSFile(bus.ROM[0xC00000:end], "/ROM/Images/SplashLogo1.gif")
	if err != nil {
		return nil, err
	}
	decoded, err := gif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return imageToYUY2(decoded), nil
}

func imageToYUY2(src image.Image) []byte {
	dst := make([]byte, videoWidth*videoHeight*2)
	bounds := src.Bounds()
	ox, oy := (videoWidth-bounds.Dx())/2, (videoHeight-bounds.Dy())/2
	for y := 0; y < videoHeight; y++ {
		for x := 0; x < videoWidth; x += 2 {
			r0, g0, b0 := splashRGB(src, bounds, x-ox, y-oy)
			r1, g1, b1 := splashRGB(src, bounds, x+1-ox, y-oy)
			y0, u0, v0 := rgbToYUV(r0, g0, b0)
			y1, u1, v1 := rgbToYUV(r1, g1, b1)
			off := (y*videoWidth + x) * 2
			dst[off], dst[off+1], dst[off+2], dst[off+3] = y0, byte((int(u0)+int(u1))/2), y1, byte((int(v0)+int(v1))/2)
		}
	}
	return dst
}

func splashRGB(src image.Image, bounds image.Rectangle, x, y int) (byte, byte, byte) {
	if x < 0 || y < 0 || x >= bounds.Dx() || y >= bounds.Dy() {
		return 0, 0, 0
	}
	r, g, b, a := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
	return byte((r * a) >> 24), byte((g * a) >> 24), byte((b * a) >> 24)
}

func rgbToYUV(r, g, b byte) (byte, byte, byte) {
	ri, gi, bi := int(r), int(g), int(b)
	return clampByte(((66*ri + 129*gi + 25*bi + 128) >> 8) + 16), clampByte(((-38*ri - 74*gi + 112*bi + 128) >> 8) + 128), clampByte(((112*ri - 94*gi - 18*bi + 128) >> 8) + 128)
}

func clampByte(value int) byte {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return byte(value)
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
