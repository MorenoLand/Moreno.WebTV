package main

import "testing"

func TestRAMMirrorsAcrossLowPhysicalAperture(t *testing.T) {
	bus := NewBus()

	bus.Write32(0xA0000000, 0x4A6F6542)

	if got := bus.Read32(0xA0800000); got != 0x4A6F6542 {
		t.Fatalf("RAM mirror read got 0x%08X, want 0x4A6F6542", got)
	}
	bus.Write32(0xA0800004, 0x11223344)
	if got := bus.Read32(0xA0000004); got != 0x11223344 {
		t.Fatalf("RAM mirror write got 0x%08X, want 0x11223344", got)
	}
}

func TestRAMMirrorRejectsOverflowedAddresses(t *testing.T) {
	if _, ok := ramMirror(0xFFFFFFFF, 4); ok {
		t.Fatal("overflowed RAM address was accepted")
	}
	if _, ok := ramMirror(RAM_APERTURE-3, 4); ok {
		t.Fatal("RAM access past the aperture was accepted")
	}
}

func TestEXLKernelLowRAMBypassesTLBWhenBEVCleared(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu
	cpu.COP0[Status] = 0x00000002 // EXL=1, BEV=0

	bus.Write32(0x000DBB98, 0x12345678)

	if cpu.tlbMiss {
		t.Fatalf("EXL low RAM write raised a TLB miss")
	}
	if got := bus.Read32(0x800DBB98); got != 0x12345678 {
		t.Fatalf("EXL low RAM write stored 0x%08X, want 0x12345678", got)
	}
}

func TestExpansionProbeSignature(t *testing.T) {
	bus := NewBus()

	if got := bus.Read8(0xA4FFFFF3); got != 2 {
		t.Fatalf("expansion probe byte got 0x%02X, want 0x02", got)
	}
}
