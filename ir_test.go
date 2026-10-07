package main

import "testing"

func TestIRTransitionTimingAndFIFOOrder(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x4020, 1185)
	a.Write(0x4024, 10)
	a.Write(0x402C, 0x3083)
	a.QueueIRTransitions([]uint16{0xBE8, 0xA0, 0x828})
	if a.IRNextAt != 1185000 || a.Read(0x4028) != 0 {
		t.Fatal("transition arrived before its sample interval")
	}
	a.Tick(1184999)
	if a.Read(0x4028) != 0 {
		t.Fatal("transition arrived one cycle early")
	}
	a.Tick(1)
	if got := a.Read(0x4028); got != 0x1BE8 {
		t.Fatalf("first transition=%04X, want 1BE8", got)
	}
	if a.IRNextAt != 1374600 {
		t.Fatalf("second transition due=%d, want 1374600", a.IRNextAt)
	}
	a.Tick(237000)
	if got := a.Read(0x4028); got != 0x20A0 {
		t.Fatalf("second transition=%04X, want 20A0", got)
	}
	if got := a.Read(0x4028); got != 0x1828 {
		t.Fatalf("third transition=%04X, want 1828", got)
	}
	if a.IRNextAt != 0 || a.Read(0x4028) != 0 {
		t.Fatal("drained waveform remained active")
	}
}
func TestIRRegistersResetAndDisabledQueue(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x4020, 0xFFFF04A1)
	a.Write(0x4024, 0xFFFFFF0A)
	a.Write(0x402C, 0x3083)
	if a.Read(0x4020) != 1185 || a.Read(0x4024) != 10 || a.Read(0x402C) != 0x81 {
		t.Fatal("IR register masks or reset pulse are incorrect")
	}
	a.Write(0x402C, 0x80)
	a.QueueIRTransitions([]uint16{0xBE8, 0xA0})
	a.Tick(2000000)
	if a.IRCount != 0 || len(a.IRPending) != 2 || a.IRNextAt != 0 {
		t.Fatal("disabled receiver consumed queued waveform")
	}
	a.Write(0x402C, 0x81)
	if a.IRNextAt != 3185000 {
		t.Fatalf("resumed transition due=%d, want 3185000", a.IRNextAt)
	}
	a.Tick(1185000)
	if a.IRCount != 1 || len(a.IRPending) != 1 {
		t.Fatal("enabled receiver did not resume queued waveform")
	}
	a.Write(0x402C, 0x3083)
	if a.Read(0x4028) != 0 || a.IRCount != 0 || len(a.IRPending) != 0 || a.IRNextAt != 0 || a.Read(0x402C) != 0x81 {
		t.Fatal("reset pulse did not flush FIFO and pending waveform")
	}
	a.QueueIRTransitions([]uint16{0xA0})
	a.Tick(189600)
	if a.Read(0x4028) != 0x10A0 {
		t.Fatal("receiver remained held in reset")
	}
}
func TestIRThresholdMasksAndStickyInterrupt(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x4020, 1)
	a.Write(0x402C, 0x3083)
	a.QueueIRTransitions([]uint16{1, 1, 1, 1, 1, 1, 1, 1})
	a.Tick(7)
	if a.DevIntStat&4 != 0 {
		t.Fatal("IRQ asserted below threshold eight")
	}
	a.Tick(1)
	if a.DevIntStat&4 == 0 || a.BusIntStat&BUS_INT_DEVICE != 0 || a.PendingIP2() {
		t.Fatal("threshold IRQ did not respect disabled device mask")
	}
	a.Write(0x007C, 4)
	if a.BusIntStat&BUS_INT_DEVICE == 0 || !a.PendingIP2() {
		t.Fatal("firmware device enable did not assert IP2 independently of bus mask")
	}
	a.Write(0x000C, BUS_INT_DEVICE)
	if !a.PendingIP2() {
		t.Fatal("enabled device IRQ did not assert IP2")
	}
	a.Write(0x0174, 4)
	if !a.PendingIP2() {
		t.Fatal("acknowledging a still-qualified FIFO lost its IRQ")
	}
	for i := 0; i < 8; i++ {
		a.Read(0x4028)
	}
	if a.DevIntStat&4 == 0 || !a.PendingIP2() {
		t.Fatal("draining FIFO cleared sticky IRQ")
	}
	a.Write(0x017C, 4)
	if a.DevIntStat&4 == 0 || a.BusIntStat&BUS_INT_DEVICE != 0 || a.PendingIP2() {
		t.Fatal("masking device cleared sticky status or retained IP2")
	}
	a.Write(0x007C, 4)
	a.Write(0x0108, BUS_INT_DEVICE)
	if !a.PendingIP2() {
		t.Fatal("bus acknowledge lost enabled sticky device IRQ")
	}
	a.Write(0x0174, 4)
	if a.DevIntStat&4 != 0 || a.BusIntStat&BUS_INT_DEVICE != 0 || a.PendingIP2() {
		t.Fatal("device acknowledge did not clear drained IRQ")
	}
	a.Write(0x0078, 0x40)
	if a.BusIntStat&BUS_INT_DEVICE != 0 {
		t.Fatal("disabled non-IR device source asserted parent IRQ")
	}
	a.Write(0x007C, 0x40)
	if !a.PendingIP2() {
		t.Fatal("enabled non-IR device source did not assert parent IRQ")
	}
	a.Write(0x0174, 0x40)
	if a.PendingIP2() {
		t.Fatal("non-IR device acknowledge did not clear parent IRQ")
	}
}
func TestIRWaveformSchedulingAndFIFOCountLimit(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x4020, 1185)
	a.Write(0x402C, 0x3083)
	transitions := remoteIRTransitions(0x2E)
	if len(transitions) != 127 {
		t.Fatalf("remote waveform length=%d, want 127", len(transitions))
	}
	a.QueueIRTransitions(transitions)
	for i, want := range transitions {
		a.Tick(uint64(want&0x7FF) * 1185)
		if a.IRCount != 1 {
			t.Fatalf("transition %d dumped pending waveform into FIFO", i)
		}
		if got := a.Read(0x4028); got != 0x1000|uint32(want) {
			t.Fatalf("transition %d=%04X, want %04X", i, got, 0x1000|uint32(want))
		}
	}
	if a.IRNextAt != 0 || len(a.IRPending) != 0 {
		t.Fatal("remote waveform did not finish")
	}
	a.Write(0x4020, 1)
	a.QueueIRTransitions(transitions[:16])
	cycles := uint64(0)
	for _, transition := range transitions[:16] {
		cycles += uint64(transition & 0x7FF)
	}
	a.Tick(cycles)
	if a.IRCount != 16 {
		t.Fatalf("FIFO count=%d, want 16", a.IRCount)
	}
	if got := a.Read(0x4028); got != 0xF000|uint32(transitions[0]) {
		t.Fatalf("full FIFO encoded count=%04X, want %04X", got, 0xF000|uint32(transitions[0]))
	}
}
func TestIRThresholdLeavesPartialWaveformTailPending(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x4020, 1185)
	a.Write(0x402C, 0x3083)
	a.Write(0x007C, 4)
	a.Write(0x000C, BUS_INT_DEVICE)
	transitions := make([]uint16, 85)
	for i := range transitions {
		transitions[i] = 0x828
	}
	a.QueueIRTransitions(transitions)
	read := 0
	for range transitions {
		a.Tick(47400)
		if !a.PendingIP2() {
			continue
		}
		for a.IRCount != 0 {
			a.Read(0x4028)
			read++
		}
		a.Write(0x0174, 4)
	}
	if read != 80 || a.IRCount != 5 || a.PendingIP2() || len(a.IRPending) != 0 {
		t.Fatalf("tail read=%d FIFO=%d pending=%d IRQ=%v", read, a.IRCount, len(a.IRPending), a.PendingIP2())
	}
	a.Tick(1185 * 2047 * 8)
	if a.IRCount != 5 || a.PendingIP2() {
		t.Fatal("model synthesized an unverified idle-overflow interrupt")
	}
}
func TestCPUWaitStopsAtIRTransitionDeadline(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu
	a := bus.Asic
	a.Write(0x4020, 1185)
	a.Write(0x402C, 0x3083)
	a.Write(0x007C, 4)
	a.Write(0x000C, BUS_INT_DEVICE)
	a.Write(0x0088, 8)
	cpu.COP0[Compare] = 2000000
	a.QueueIRTransitions([]uint16{40, 40, 40, 40, 40, 40, 40, 40})
	for i := 1; i <= 8; i++ {
		cpu.waitForInterrupt()
		if a.Cycles != uint64(i)*47400 || a.IRCount != i {
			t.Fatalf("WAIT %d advanced cycles=%d FIFO=%d past IR deadline", i, a.Cycles, a.IRCount)
		}
	}
	if cpu.COP0[Cause]&0x400 == 0 || !a.PendingIP2() {
		t.Fatal("WAIT did not expose threshold IR interrupt")
	}
	if cpu.COP0[Cause]&0x8000 != 0 || a.VIDIntStat != 0 {
		t.Fatal("WAIT skipped to a later timer or video event")
	}
}
func TestIRFirmwareDeviceEnableRoutesIP2WithTimerOnlyBusMask(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu
	a := bus.Asic
	a.Write(0x000C, BUS_INT_TIMER)
	a.Write(0x4020, 0x4DD)
	a.Write(0x4024, 10)
	a.Write(0x402C, 0x3083)
	a.QueueIRTransitions([]uint16{40, 40, 40, 40, 40, 40, 40, 40})
	a.Tick(8 * 40 * 1245)
	if a.PendingIP2() {
		t.Fatal("disabled DEV source asserted IP2")
	}
	a.Write(0x007C, 4)
	if a.BusIntEnable != BUS_INT_TIMER || a.BusIntStat&BUS_INT_DEVICE == 0 || !a.PendingIP2() {
		t.Fatalf("firmware IRQ routing failed: busEn=%X busStat=%X devEn=%X devStat=%X", a.BusIntEnable, a.BusIntStat, a.DevIntEnable, a.DevIntStat)
	}
	cpu.COP0[Status] = 0x401
	if !cpu.checkInterrupts() || cpu.COP0[Cause]&0x400 == 0 {
		t.Fatal("firmware DEV enable did not reach CPU IP2")
	}
	for i := 0; i < 8; i++ {
		a.Read(0x4028)
	}
	a.Write(0x0174, 4)
	if a.PendingIP2() || a.BusIntStat&BUS_INT_DEVICE != 0 {
		t.Fatal("drain and DEV acknowledge retained IP2")
	}
}
func TestDEVInterruptRegisterAddressesAndMaskedStatus(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x0078, 0x44)
	if a.Read(0x0074) != 0 || a.Read(0x0078) != 0x44 || a.PendingIP2() {
		t.Fatal("disabled DEV status was not masked at 0074 or preserved raw at 0078")
	}
	a.Write(0x007C, 4)
	if a.Read(0x0074) != 4 || a.Read(0x0078) != 0x44 || a.Read(0x007C) != 4 || !a.PendingIP2() {
		t.Fatal("007C enable did not expose IRQ 1D mask at 0074")
	}
	a.Write(0x017C, 4)
	if a.Read(0x0074) != 0 || a.Read(0x0078) != 0x44 || a.PendingIP2() {
		t.Fatal("017C disable cleared raw status or retained masked IRQ")
	}
	a.Write(0x007C, 0x40)
	a.Write(0x0174, 4)
	if a.Read(0x0074) != 0x40 || a.Read(0x0078) != 0x40 || !a.PendingIP2() {
		t.Fatal("0174 acknowledge cleared a different pending source")
	}
	a.Write(0x0174, 0x40)
	if a.Read(0x0074) != 0 || a.Read(0x0078) != 0 || a.PendingIP2() {
		t.Fatal("0174 acknowledge did not clear DEV status")
	}
	if a.UnmodeledReadHits[0x0074] != 0 || a.UnmodeledWriteHits[0x0078] != 0 {
		t.Fatal("DEV interrupt register accesses were unmodeled")
	}
}
