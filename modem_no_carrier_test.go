package main

import "testing"

func TestModemNoCarrierQueuesDialFailure(t *testing.T) {
	bus := NewBus()
	bus.ModemNoCarrier = true
	bus.ModemStage = 4
	bus.modemWrite(0x04, 0x09, 1)
	bus.modemWrite(0x10, 0x0B, 1)
	if bus.ModemNoCarrierAt == 0 {
		t.Fatal("no-carrier result was not scheduled")
	}
	bus.Tick(modemNoCarrierDelayCycles - 1)
	if bus.ModemNoCarrierQueued {
		t.Fatal("no-carrier result queued before delay")
	}
	bus.Tick(1)
	if !bus.ModemNoCarrierQueued {
		t.Fatal("no-carrier result was not queued")
	}
	if len(bus.ModemWireRX) == 0 {
		t.Fatal("no-carrier result did not enter the modem wire queue")
	}
}
