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
	if asic.PendingIP2() {
		t.Fatalf("timer source bypassed disabled bus interrupt")
	}
	asic.BusIntEnable = BUS_INT_TIMER
	asic.BusIntStat = BUS_INT_TIMER
	if !asic.PendingIP2() {
		t.Fatalf("enabled timer bus interrupt did not assert IP2")
	}
}

func TestTimerEnableUsesBusInterruptRouting(t *testing.T) {
	asic := NewSoloASIC()
	asic.Write(0x00A4, BUS_INT_TIM_SYSTIMER)
	if asic.TimIntEnable != BUS_INT_TIM_SYSTIMER {
		t.Fatalf("timer enable=0x%08X, want only system timer", asic.TimIntEnable)
	}
	if asic.BusIntEnable&BUS_INT_TIMER == 0 {
		t.Fatalf("timer source did not enable bus timer interrupt")
	}
	if asic.PendingIP2() {
		t.Fatalf("enabling timer source asserted IP2 without pending status")
	}
}

func TestVideoInterruptTicksAndAcknowledges(t *testing.T) {
	asic := NewSoloASIC()
	asic.Write(0x0088, 0x08)
	asic.Tick(videoFramePeriodCycles - 1)
	if asic.VIDIntStat != 0 || asic.PendingIP2() {
		t.Fatalf("video interrupt fired before frame boundary")
	}
	asic.Tick(1)
	if asic.VIDIntStat&0x08 == 0 || asic.BusIntStat&BUS_INT_VIDEO == 0 || !asic.PendingIP2() {
		t.Fatalf("enabled video interrupt did not assert IP2: stat=0x%08X bus=0x%08X", asic.VIDIntStat, asic.BusIntStat)
	}
	asic.Write(0x0180, 0x08)
	if asic.VIDIntStat != 0 || asic.BusIntStat&BUS_INT_VIDEO != 0 || asic.PendingIP2() {
		t.Fatalf("video interrupt acknowledge did not clear IP2")
	}
}
