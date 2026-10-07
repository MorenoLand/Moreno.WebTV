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

func TestExpansionProbeIsUnmapped(t *testing.T) {
	t.Setenv("WEBTV_WTV_PORT_RESPONSE", "none")
	bus := NewBus()

	if got := bus.Read8(0xA4FFFFF3); got != 0xFF {
		t.Fatalf("unmapped expansion probe byte got 0x%02X, want 0xFF", got)
	}
	if got := bus.Read32(0xA4FFFFF0); got != 0xFFFFFFFF {
		t.Fatalf("unmapped expansion probe word got 0x%08X, want 0xFFFFFFFF", got)
	}
}

func TestDefaultWTVPortHasNoExternalAdapter(t *testing.T) {
	t.Setenv("WEBTV_WTV_PORT_RESPONSE", "")
	if got, ok := webTVPortResponse(webTVPortProbeAddress, 1); ok {
		t.Fatalf("default WTV byte advertises an adapter: %#x", got)
	}
	bus := NewBus()
	if got := bus.Read8(0xA4FFFFF3); got != 0xFF {
		t.Fatalf("default adapter byte = %#x, want 0xFF", got)
	}
	if got := bus.Read32(0xA4FFFFF0); got != 0xFFFFFFFF {
		t.Fatalf("default adapter word = %#x, want 0xFFFFFFFF", got)
	}
	if got, ok := webTVPortResponse(webTVPortProbeWordAddress, 4); ok {
		t.Fatalf("default WTV word advertises an adapter: %#x", got)
	}
}

func TestExpansionProbeDoesNotAliasParallelPortStatus(t *testing.T) {
	t.Setenv("WEBTV_WTV_PORT_RESPONSE", "none")
	bus := NewBus()
	bus.Asic.PPortStatus = 0x08
	if got := bus.Read8(0xA4FFFFF3); got != 0xFF {
		t.Fatalf("expansion probe aliased parallel status, got 0x%02X", got)
	}
}

func TestModemCarrierAssertsDCD(t *testing.T) {
	bus := NewBus()
	bus.ModemStage = 4

	bus.modemWrite(0x10, 0x0B, 1)
	bus.Tick(10000)

	if got := bus.ModemMSR; got != 0xBB {
		t.Fatalf("modem status got 0x%02X, want DCD/DSR/CTS with deltas 0xBB", got)
	}
}

func TestModemRXInterruptPrecedesCarrierStatus(t *testing.T) {
	bus := NewBus()
	bus.ModemStage = 4
	bus.modemWrite(0x04, 0x09, 1)
	bus.modemWrite(0x10, 3, 1)
	bus.Tick(10000)
	bus.queueModemRX([]byte{0x7E})
	if got := bus.modemRead8(0x08); got != 0x04 {
		t.Fatalf("RX interrupt identification = 0x%02X, want 0x04", got)
	}
	if got := bus.modemRead8(0x00); got != 0x7E {
		t.Fatalf("RX byte = 0x%02X, want 0x7E", got)
	}
	if got := bus.modemRead8(0x08); got != 0x00 {
		t.Fatalf("carrier interrupt identification = 0x%02X, want 0x00", got)
	}
}

func TestModemWireRXDeliversOneBytePerCharacterTime(t *testing.T) {
	bus := NewBus()
	bus.Asic.Write(0x0094, 4)
	bus.modemWrite(0x04, 1, 1)
	bus.ModemStage = 4
	bus.queueModemWireRX([]byte{0x41, 0x42})
	if len(bus.ModemRX) != 0 || len(bus.ModemWireRX) != 2 {
		t.Fatalf("wire RX was delivered synchronously: ready=%d pending=%d, want 0/2", len(bus.ModemRX), len(bus.ModemWireRX))
	}
	bus.Tick(modemCharCycles - 1)
	if len(bus.ModemRX) != 0 {
		t.Fatalf("first wire RX byte arrived early: ready=%d", len(bus.ModemRX))
	}
	bus.Tick(1)
	if got := bus.modemRead8(0x00); got != 0x41 {
		t.Fatalf("first wire RX byte = 0x%02X, want 0x41", got)
	}
	bus.Tick(modemCharCycles)
	if got := bus.modemRead8(0x00); got != 0x42 {
		t.Fatalf("second wire RX byte = 0x%02X, want 0x42", got)
	}
}
