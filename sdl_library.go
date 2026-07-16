package main

import (
	"os"
	"path/filepath"
)

func sdl2DLLPath() string {
	if path := os.Getenv("SDL2_DLL"); path != "" {
		return path
	}
	path := filepath.Join(".tools", "sdl2", "SDL2-2.30.10", "x86_64-w64-mingw32", "bin", "SDL2.dll")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return "SDL2.dll"
}
