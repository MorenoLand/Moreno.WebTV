package main

import (
	"bytes"
	"testing"
	"time"
)

func TestVideoGeometryMatchesFirmwareFrameSize(t *testing.T) {
	if got := videoWidth * videoHeight * 2; got != 0x72D80 {
		t.Fatalf("YUY2 frame size=0x%X, want firmware VID_NSIZE 0x72D80", got)
	}
}

func TestStableVideoFrameWaitsForCompletedRedraw(t *testing.T) {
	splash := []byte{16, 128, 16, 128}
	frames := stableVideoFrame{presented: splash}
	start := time.Unix(0, 0)
	partial := []byte{32, 128, 16, 128}
	complete := []byte{32, 128, 32, 128}
	if got := frames.update(partial, start); !bytes.Equal(got, splash) {
		t.Fatalf("partial redraw was presented: %v", got)
	}
	if got := frames.update(complete, start.Add(videoStableDuration)); !bytes.Equal(got, splash) {
		t.Fatalf("changing redraw was presented: %v", got)
	}
	if got := frames.update(complete, start.Add(2*videoStableDuration)); !bytes.Equal(got, complete) {
		t.Fatalf("stable redraw was not presented: %v", got)
	}
}

func TestStableVideoFrameRejectsMagentaScratchBuffer(t *testing.T) {
	splash := []byte{16, 128, 16, 128}
	frames := stableVideoFrame{presented: splash}
	start := time.Unix(0, 0)
	magenta := bytes.Repeat([]byte{140, 202, 140, 192}, 64)
	frames.update(magenta, start)
	if got := frames.update(magenta, start.Add(2*videoStableDuration)); !bytes.Equal(got, splash) {
		t.Fatalf("magenta scratch buffer was presented: %v", got[:4])
	}
}

func TestStableVideoFrameRejectsMostlyBlankScanout(t *testing.T) {
	splash := []byte{16, 128, 16, 128}
	frames := stableVideoFrame{presented: splash}
	start := time.Unix(0, 0)
	blank := bytes.Repeat([]byte{16, 128, 16, 128}, 64)
	frames.update(blank, start)
	if got := frames.update(blank, start.Add(2*videoStableDuration)); !bytes.Equal(got, splash) {
		t.Fatalf("blank scanout was presented: %v", got[:4])
	}
}

func TestStableVideoFrameRejectsTopBandOnlyScanout(t *testing.T) {
	splash := []byte{16, 128, 16, 128}
	frames := stableVideoFrame{presented: splash}
	start := time.Unix(0, 0)
	static := bytes.Repeat([]byte{80, 202, 80, 192}, 6)
	static = append(static, bytes.Repeat([]byte{16, 128, 16, 128}, 58)...)
	frames.update(static, start)
	if got := frames.update(static, start.Add(2*videoStableDuration)); !bytes.Equal(got, splash) {
		t.Fatalf("top-band scanout was presented: %v", got[:4])
	}
}

func TestVideoFrameBytesUsesActiveVideoDMA(t *testing.T) {
	bus := NewBus()
	bus.RAM[0x1000] = 0x55
	bus.Asic.Write(0x300C, 0x80001000)
	bus.Asic.Write(0x3010, videoWidth*videoHeight*2)
	bus.Asic.Write(0x3014, 0x1F)
	data, ok := videoFrameBytes(bus)
	if !ok {
		t.Fatal("armed video frame should fit in RAM")
	}
	if len(data) != videoWidth*videoHeight*2 || data[0] != bus.RAM[0x1000] {
		t.Fatal("frame data does not start at the active video buffer")
	}
}

func TestVideoFrameBytesRejectsResetVIDNStart(t *testing.T) {
	bus := NewBus()
	if _, ok := videoFrameBytes(bus); ok {
		t.Fatal("reset VID_NSTART must not expose low RAM as video")
	}
}
