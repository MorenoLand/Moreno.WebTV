package main

import "testing"

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
