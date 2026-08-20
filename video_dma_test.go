package main

import "testing"

func TestSoloASICVideoDMAReadsConfiguredFrame(t *testing.T) {
	a := NewSoloASIC()
	want := []byte{0x00, 0x11, 0x22, 0x33}
	reads := 0
	a.VideoMemoryRead = func(addr uint32, dst []byte) bool {
		reads++
		if addr != 0x120 {
			t.Fatalf("video DMA address = 0x%X, want 0x120", addr)
		}
		copy(dst, want)
		return true
	}
	a.Write(0x300C, 0x80000120)
	a.Write(0x3010, uint32(len(want)))
	a.Write(0x3014, 0x1F)
	if got := a.Read(0x3000); got != 0x80000120 {
		t.Fatalf("VID_CSTART = 0x%X, want 0x80000120", got)
	}
	if got := a.Read(0x3004); got != uint32(len(want)) {
		t.Fatalf("VID_CSIZE = 0x%X, want %d", got, len(want))
	}
	if got := a.Read(0x3008); got != 0 {
		t.Fatalf("VID_CCNT = 0x%X, want 0", got)
	}
	if got := a.Read(0x300C); got != 0x80000120 {
		t.Fatalf("VID_NSTART = 0x%X, want 0x80000120", got)
	}
	if got := a.Read(0x3010); got != uint32(len(want)) {
		t.Fatalf("VID_NSIZE = 0x%X, want %d", got, len(want))
	}
	if got := a.Read(0x3014); got != 0x1F {
		t.Fatalf("VID_DMACNTL = 0x%X, want 0x1F", got)
	}
	if reads != 1 {
		t.Fatalf("video DMA reads = %d, want 1", reads)
	}
	if len(a.VideoFrame) != len(want) {
		t.Fatalf("video frame length = %d, want %d", len(a.VideoFrame), len(want))
	}
	for i, v := range want {
		if a.VideoFrame[i] != v {
			t.Fatalf("video frame byte %d = 0x%X, want 0x%X", i, a.VideoFrame[i], v)
		}
	}
}

func TestSoloASICVideoDMARequiresControl(t *testing.T) {
	a := NewSoloASIC()
	a.VideoMemoryRead = func(uint32, []byte) bool { t.Fatal("video DMA read while disabled"); return false }
	a.Write(0x300C, 0x80000120)
	a.Write(0x3010, 4)
	a.Write(0x3014, 0)
	if a.Read(0x3000) != 0 || len(a.VideoFrame) != 0 {
		t.Fatalf("video frame allocated while DMA disabled")
	}
}
