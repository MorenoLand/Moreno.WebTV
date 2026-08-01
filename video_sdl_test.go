package main

import "testing"

func TestVideoGeometryMatchesFirmwareFrameSize(t *testing.T) {
	if got := videoWidth * videoHeight * 2; got != 0x72D80 {
		t.Fatalf("YUY2 frame size=0x%X, want firmware VID_NSIZE 0x72D80", got)
	}
}

func TestVideoFrameBytesUsesVIDNStart(t *testing.T) {
	bus := NewBus()
	bus.Asic.VIDNStart = 0x80001000
	data, ok := videoFrameBytes(bus)
	if !ok {
		t.Fatal("VID_NSTART frame should fit in RAM")
	}
	if len(data) != videoWidth*videoHeight*2 || &data[0] != &bus.RAM[0x1000] {
		t.Fatal("frame data does not start at VID_NSTART")
	}
}
