package main

import "testing"

func TestRIOPollStatusWaitsAfterAck(t *testing.T) {
	asic := NewSoloASIC()

	if got := asic.Read(0x90A0); got != 1 {
		t.Fatalf("initial RIO poll status=0x%08X, want ready", got)
	}
	asic.Write(0x90A8, 1)
	if got := asic.Read(0x90A0); got != 0 {
		t.Fatalf("RIO poll status immediately after ack=0x%08X, want not ready", got)
	}
	asic.Tick(rioPollPeriodCycles - 1)
	if got := asic.Read(0x90A0); got != 0 {
		t.Fatalf("RIO poll status before period elapsed=0x%08X, want not ready", got)
	}
	asic.Tick(1)
	if got := asic.Read(0x90A0); got != 1 {
		t.Fatalf("RIO poll status after period elapsed=0x%08X, want ready", got)
	}
}

func TestPendingIP2RequiresEnabledInterruptSource(t *testing.T) {
	asic := NewSoloASIC()

	asic.BusIntStat = BUS_INT_RIO
	if asic.PendingIP2() {
		t.Fatalf("disabled bus interrupt asserted IP2")
	}
	asic.BusIntEnable = BUS_INT_RIO
	if !asic.PendingIP2() {
		t.Fatalf("enabled bus interrupt did not assert IP2")
	}

	asic.BusIntStat = 0
	asic.BusIntEnable = 0
	asic.TimIntStat = BUS_INT_TIM_SYSTIMER
	if asic.PendingIP2() {
		t.Fatalf("disabled timer interrupt asserted IP2")
	}
	asic.TimIntEnable = BUS_INT_TIM_SYSTIMER
	if !asic.PendingIP2() {
		t.Fatalf("enabled timer interrupt did not assert IP2")
	}
}
