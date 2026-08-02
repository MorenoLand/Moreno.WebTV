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

func TestModemRXUsesRIOBusInterrupt(t *testing.T) {
	bus := NewBus()
	bus.modemWrite(0x04, 1, 1)
	bus.queueModemRX([]byte{0x41})
	if bus.Asic.BusIntStat&BUS_INT_RIO == 0 {
		t.Fatalf("modem RX did not assert RIO bus interrupt: stat=0x%08X", bus.Asic.BusIntStat)
	}
	if bus.Asic.BusIntStat&(1<<6) != 0 {
		t.Fatalf("modem RX incorrectly asserted audio bus interrupt: stat=0x%08X", bus.Asic.BusIntStat)
	}
}

func TestModemUARTReadyStatusInterrupt(t *testing.T) {
	bus := NewBus()
	bus.ModemStage = 4
	bus.modemWrite(0x04, 0x09, 1)
	bus.modemWrite(0x10, 0x0B, 1)
	bus.modemWrite(0x10, 0x0B, 1)
	bus.Tick(9999)
	if bus.Asic.BusIntStat&BUS_INT_RIO != 0 {
		t.Fatalf("modem status interrupt asserted before ready delay")
	}
	bus.Tick(1)
	if got := bus.modemRead8(0x08); got != 0x00 {
		t.Fatalf("modem status interrupt identification = 0x%02X, want 0x00", got)
	}
	if bus.Asic.BusIntStat&BUS_INT_RIO == 0 {
		t.Fatalf("modem status did not assert RIO interrupt: stat=0x%08X", bus.Asic.BusIntStat)
	}
	bus.Asic.Write(0x018C, 4)
	bus.Tick(1)
	if bus.Asic.BusIntStat&BUS_INT_RIO == 0 {
		t.Fatalf("pending modem status did not reassert device interrupt: stat=0x%08X", bus.Asic.BusIntStat)
	}
	if got := bus.modemRead8(0x18); got != 0x33 {
		t.Fatalf("modem ready MSR = 0x%02X, want 0x33", got)
	}
	if got := bus.modemRead8(0x08); got != 0x01 {
		t.Fatalf("cleared modem interrupt identification = 0x%02X, want 0x01", got)
	}
	if bus.Asic.BusIntStat&BUS_INT_RIO != 0 {
		t.Fatalf("modem status acknowledge left device interrupt asserted: stat=0x%08X", bus.Asic.BusIntStat)
	}
	if got := bus.modemRead8(0x14); got != 0x60 {
		t.Fatalf("idle modem LSR = 0x%02X, want 0x60", got)
	}
}

func TestRIOInterruptEnableRaisesIP2(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x0094, 4)
	a.RIOIntStat = 4
	a.assertBusIRQ()
	if !a.PendingIP2() {
		t.Fatal("enabled RIO interrupt did not raise IP2")
	}
}

func TestModemUARTDivisorLatch(t *testing.T) {
	bus := NewBus()
	bus.modemWrite(0x0C, 0x80, 1)
	bus.modemWrite(0x00, 1, 1)
	bus.modemWrite(0x04, 0, 1)
	if bus.ModemDLL != 1 || bus.ModemDLM != 0 || bus.ModemIER != 0 || len(bus.ModemTXLine) != 0 {
		t.Fatalf("divisor latch write leaked into UART data state: DLL=%d DLM=%d IER=%d TX=% X", bus.ModemDLL, bus.ModemDLM, bus.ModemIER, bus.ModemTXLine)
	}
}

func TestParallelPortRegistersFollowSoloMapping(t *testing.T) {
	a := NewSoloASIC()
	if got := a.Read(0x4200); got != 0 {
		t.Fatalf("disabled parallel data=0x%08X, want zero", got)
	}
	a.Write(0x422C, 1)
	a.Write(0x4200, 0x1234)
	if got := a.Read(0x4200); got != 0x34 {
		t.Fatalf("parallel data=0x%08X, want 0x34", got)
	}
	a.Write(0x4204, 0x1F)
	if got := a.PPortCtrl; got != 0x2F {
		t.Fatalf("parallel LPT control=0x%08X, want 0x2F", got)
	}
	if got := a.Read(0x4204); got != 0x1F {
		t.Fatalf("parallel ASIC control=0x%08X, want 0x1F", got)
	}
	a.PPortStatus = 0xF8
	if got := a.Read(0x4208); got != 0x1F {
		t.Fatalf("parallel ASIC status=0x%08X, want 0x1F", got)
	}
	a.Write(0x4224, 0x05)
	a.Write(0x4224, 0x08)
	if got := a.Read(0x4228); got != 0x0D {
		t.Fatalf("parallel interrupt status=0x%08X, want 0x0D", got)
	}
	a.Write(0x4228, 0x04)
	if got := a.Read(0x4224); got != 0x09 {
		t.Fatalf("parallel interrupt clear=0x%08X, want 0x09", got)
	}
}

func TestParallelPortDisableGatesDataAndStatus(t *testing.T) {
	a := NewSoloASIC()
	a.PPortData = 0xAA
	a.PPortStatus = 0xF8
	if got := a.Read(0x4200); got != 0 || a.Read(0x4208) != 0 {
		t.Fatalf("disabled parallel port exposed data or status")
	}
	a.Write(0x422C, 1)
	a.Write(0x422C, 0)
	if got := a.Read(0x422C); got != 0 {
		t.Fatalf("parallel enable readback=0x%08X, want zero", got)
	}
}
