package main

import "testing"

func TestYUVToRGBNeutralRange(t *testing.T) {
	r, g, b := yuvToRGB(16, 128, 128)
	if r != 0 || g != 0 || b != 0 {
		t.Fatalf("black YUV converted to %d,%d,%d", r, g, b)
	}
	r, g, b = yuvToRGB(235, 128, 128)
	if r != 255 || g != 255 || b != 255 {
		t.Fatalf("white YUV converted to %d,%d,%d", r, g, b)
	}
}
