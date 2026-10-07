package main

import "testing"

func TestModemDTRWinkPreservesMCRAndRestoresCTS(t *testing.T) {
	bus := NewBus()
	bus.ModemStage = 4
	bus.modemWrite(0x10, 0x0B, 1)
	bus.Tick(10000)
	if got := bus.modemRead8(0x10); got != 0x0B {
		t.Fatalf("MCR=%02X, want 0B", got)
	}
	bus.modemWrite(0x10, uint32(bus.modemRead8(0x10)&^1), 1)
	if got := bus.modemRead8(0x10); got != 0x0A {
		t.Fatalf("DTR wink lost RTS/OUT2: %02X", got)
	}
	bus.modemWrite(0x10, uint32(bus.modemRead8(0x10)|1), 1)
	bus.Tick(10000)
	if bus.ModemMCR != 0x0B || bus.modemRead8(0x18)&0x10 == 0 {
		t.Fatal("DTR restoration failed to preserve readiness")
	}
}

func TestModemTHREInterruptAcknowledgesWithoutClearingRIO(t *testing.T) {
	bus := NewBus()
	bus.Asic.Write(0x0094, 4)
	bus.modemWrite(0x04, 2, 1)
	if got := bus.modemRead8(0x14); got != 0x60 {
		t.Fatalf("idle LSR=%02X, want THRE and TEMT", got)
	}
	if got := bus.modemRead8(0x08); got != 2 {
		t.Fatalf("TX IIR=%02X, want 02", got)
	}
	if got := bus.modemRead8(0x08); got != 1 {
		t.Fatalf("acknowledged TX IIR=%02X, want 01", got)
	}
	if bus.Asic.RIOIntStat&4 == 0 {
		t.Fatal("IIR read cleared sticky RIO status")
	}
	bus.Asic.Write(0x018C, 4)
	bus.Tick(1)
	if bus.Asic.RIOIntStat&4 != 0 {
		t.Fatal("acknowledged empty transmitter reasserted without an event")
	}
	bus.modemWrite(0, 'A', 1)
	if got := bus.modemRead8(0x08); got != 2 {
		t.Fatalf("completed THR write did not rearm TX interrupt: %02X", got)
	}
}

func TestModemInterruptPriorityKeepsTXPendingBehindRX(t *testing.T) {
	bus := NewBus()
	bus.modemWrite(0x04, 0x0B, 1)
	bus.ModemMSR = 0x11
	bus.queueModemRX([]byte{'R'})
	if bus.modemRead8(0x08) != 4 {
		t.Fatal("RX did not have priority over TX and modem status")
	}
	if bus.modemRead8(0) != 'R' || bus.modemRead8(0x08) != 2 {
		t.Fatal("RX service lost pending TX interrupt")
	}
	if bus.modemRead8(0x08) != 0 {
		t.Fatal("TX acknowledgment lost lower-priority modem status")
	}
	bus.modemRead8(0x18)
	if bus.modemRead8(0x08) != 1 {
		t.Fatal("all acknowledged sources did not become idle")
	}
}

func TestModemTHREMaskAndDivisorLatch(t *testing.T) {
	bus := NewBus()
	bus.modemWrite(0x04, 2, 1)
	bus.modemRead8(0x08)
	bus.modemWrite(0x0C, 0x80, 1)
	bus.modemWrite(0, 1, 1)
	bus.modemWrite(0x04, 0, 1)
	if bus.modemRead8(0x08) != 1 || bus.ModemIER != 2 {
		t.Fatal("divisor writes changed TX interrupt state")
	}
	bus.modemWrite(0x0C, 3, 1)
	bus.modemWrite(0, 'T', 1)
	bus.modemWrite(0x04, 0, 1)
	if bus.modemRead8(0x08) != 1 {
		t.Fatal("disabled TX interrupt remained identified")
	}
	bus.modemWrite(0x04, 2, 1)
	if bus.modemRead8(0x08) != 2 {
		t.Fatal("enabling an empty transmitter failed to assert THRE")
	}
}
