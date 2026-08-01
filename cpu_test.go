package main

import (
	"encoding/binary"
	"testing"
)

func TestLoadTLBMissDoesNotWriteRegister(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.Reset()
	cpu.COP0[Status] = 0x00400002 // BEV=1, EXL=1, ERL=0
	cpu.PC = 0x80000000
	cpu.Regs[2] = 0x12345678
	binary.BigEndian.PutUint32(bus.RAM[0:], 0x8C020008) // LW r2, 8(r0)

	if err := cpu.Step(); err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if cpu.Regs[2] != 0x12345678 {
		t.Fatalf("TLB-missing load wrote r2=0x%08X", uint32(cpu.Regs[2]))
	}
	if cpu.COP0[EPC] != 0x80000000 {
		t.Fatalf("TLB miss EPC=0x%08X, want faulting instruction PC 0x80000000", cpu.COP0[EPC])
	}
	if cpu.PC != 0xBFC00200 {
		t.Fatalf("TLB miss PC=0x%08X, want exception vector 0xBFC00200", cpu.PC)
	}
}

func TestFirmwareKSEG2RAMAlias(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	phys, ok := cpu.Translate(0xC007E960)
	if !ok || phys != 0x0007E960 {
		t.Fatalf("KSEG2 RAM alias phys=0x%08X ok=%v, want 0x0007E960 true", phys, ok)
	}
}

func TestERETRestoresEPCAndClearsEXL(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.Reset()
	cpu.PC = 0x80000000
	cpu.COP0[Status] = 0x00000002
	cpu.COP0[EPC] = 0x80001234
	binary.BigEndian.PutUint32(bus.RAM[0:], 0x42000018) // ERET

	if err := cpu.Step(); err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if cpu.PC != 0x80001234 {
		t.Fatalf("ERET PC=0x%08X, want EPC 0x80001234", cpu.PC)
	}
	if cpu.COP0[Status]&0x02 != 0 {
		t.Fatalf("ERET left EXL set in Status=0x%08X", cpu.COP0[Status])
	}
}

func TestJALRExecutesDelaySlotBeforeTarget(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.Reset()
	cpu.PC = 0x80000000
	cpu.Regs[8] = 0x80000010
	cpu.Regs[4] = 1
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x0100F809) // JALR ra, t0
	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0x24840004) // ADDIU a0, a0, 4
	binary.BigEndian.PutUint32(bus.RAM[0x10:], 0x00000000) // target NOP

	if err := cpu.Step(); err != nil {
		t.Fatalf("JALR step returned error: %v", err)
	}
	if err := cpu.Step(); err != nil {
		t.Fatalf("delay slot step returned error: %v", err)
	}
	if cpu.Regs[4] != 5 {
		t.Fatalf("JALR skipped delay slot, a0=%d", cpu.Regs[4])
	}
	if cpu.PC != 0x80000010 {
		t.Fatalf("after delay slot PC=0x%08X, want jump target 0x80000010", cpu.PC)
	}
	if cpu.Regs[31] != 0xFFFFFFFF80000008 {
		t.Fatalf("JALR link ra=0x%016X, want sign-extended 0xFFFFFFFF80000008", cpu.Regs[31])
	}
}

func TestInterruptInDelaySlotSetsBranchEPCAndBD(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.Reset()
	cpu.PC = 0x80000000
	cpu.Regs[8] = 0x80000100
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x01000008)  // JR t0
	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0x00000000)  // delay slot NOP
	binary.BigEndian.PutUint32(bus.RAM[0x100:], 0x00000000) // target NOP

	if err := cpu.Step(); err != nil {
		t.Fatalf("JR step returned error: %v", err)
	}
	cpu.COP0[Status] = 0x00408001 // BEV=1, IE=1, IM7 enabled
	cpu.COP0[Cause] = 0x00008000  // IP7 pending
	if err := cpu.Step(); err != nil {
		t.Fatalf("delay-slot interrupt step returned error: %v", err)
	}
	if cpu.COP0[EPC] != 0x80000000 {
		t.Fatalf("delay-slot interrupt EPC=0x%08X, want branch PC 0x80000000", cpu.COP0[EPC])
	}
	if cpu.COP0[Cause]&0x80000000 == 0 {
		t.Fatalf("delay-slot interrupt did not set Cause.BD: Cause=0x%08X", cpu.COP0[Cause])
	}
	if cpu.PC != 0xBFC00380 {
		t.Fatalf("delay-slot interrupt PC=0x%08X, want BEV vector 0xBFC00380", cpu.PC)
	}
}

func TestBranchLikelyNotTakenSkipsDelaySlot(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.Reset()
	cpu.PC = 0x80000000
	cpu.Regs[8] = 7
	cpu.Regs[9] = 7
	cpu.Regs[4] = 1
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x55090001) // BNEL t0,t1,+1
	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0x24840004) // ADDIU a0,a0,4
	binary.BigEndian.PutUint32(bus.RAM[0x08:], 0x24840001) // ADDIU a0,a0,1

	if err := cpu.Step(); err != nil {
		t.Fatalf("BNEL step returned error: %v", err)
	}
	if cpu.PC != 0x80000008 {
		t.Fatalf("BNEL not-taken PC=0x%08X, want delay slot skipped to 0x80000008", cpu.PC)
	}
	if err := cpu.Step(); err != nil {
		t.Fatalf("post-BNEL step returned error: %v", err)
	}
	if cpu.Regs[4] != 2 {
		t.Fatalf("BNEL not-taken executed delay slot, a0=%d", cpu.Regs[4])
	}
}

func TestWordResultsAreSignExtendedForBranches(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.Reset()
	cpu.PC = 0x80000000
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x2408FFFF) // ADDIU t0,r0,-1
	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0x05010001) // BGEZ t0,+1
	binary.BigEndian.PutUint32(bus.RAM[0x08:], 0x24040001) // ADDIU a0,r0,1
	binary.BigEndian.PutUint32(bus.RAM[0x0C:], 0x24040002) // ADDIU a0,r0,2

	if err := cpu.Step(); err != nil {
		t.Fatalf("ADDIU step returned error: %v", err)
	}
	if cpu.Regs[8] != 0xFFFFFFFFFFFFFFFF {
		t.Fatalf("ADDIU -1 wrote 0x%016X, want sign-extended -1", cpu.Regs[8])
	}
	if err := cpu.Step(); err != nil {
		t.Fatalf("BGEZ step returned error: %v", err)
	}
	if err := cpu.Step(); err != nil {
		t.Fatalf("post-BGEZ step returned error: %v", err)
	}
	if cpu.Regs[4] != 1 {
		t.Fatalf("BGEZ treated sign-extended -1 as non-negative, a0=%d", cpu.Regs[4])
	}
}

func TestUnalignedLoadWordPairBigEndian(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	copy(bus.RAM[0x100:], []byte{0x11, 0x22, 0x33, 0x44, 0x55})
	cpu.Regs[2] = 0xAAAAAAAA

	cpu.execLWL(2, 0x80000101)
	cpu.execLWR(2, 0x80000104)

	if got := uint32(cpu.Regs[2]); got != 0x22334455 {
		t.Fatalf("LWL/LWR loaded 0x%08X, want 0x22334455", got)
	}
}

func TestUnalignedStoreWordPairBigEndian(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	for i := 0; i < 8; i++ {
		bus.RAM[0x100+i] = 0xAA
	}
	cpu.Regs[2] = 0x11223344

	cpu.execSWL(2, 0x80000101)
	cpu.execSWR(2, 0x80000104)

	got := bus.RAM[0x101:0x105]
	want := []byte{0x11, 0x22, 0x33, 0x44}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SWL/SWR bytes=% X, want % X", got, want)
		}
	}
}

func TestSLTUsesSigned32BitComparison(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	cpu.Regs[8] = 0xFFFFFFFF
	cpu.Regs[9] = 1
	cpu.execSpecial(0x80000000, 0x0109502A, 8, 9, 10, 0, cpu.Regs[8], cpu.Regs[9]) // SLT t2,t0,t1

	if cpu.Regs[10] != 1 {
		t.Fatalf("SLT treated 0xFFFFFFFF as unsigned/64-bit, got %d", cpu.Regs[10])
	}
}

func TestByteAndHalfwordLoadSignedness(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	bus.RAM[0x100] = 0x80
	bus.RAM[0x102] = 0x80
	bus.RAM[0x103] = 0x01
	cpu.PC = 0x80000000

	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x80020100) // LB r2,0x100(r0)
	if err := cpu.Step(); err != nil {
		t.Fatalf("LB step returned error: %v", err)
	}
	if got := uint32(cpu.Regs[2]); got != 0xFFFFFF80 {
		t.Fatalf("LB got 0x%08X, want 0xFFFFFF80", got)
	}

	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0x90030100) // LBU r3,0x100(r0)
	if err := cpu.Step(); err != nil {
		t.Fatalf("LBU step returned error: %v", err)
	}
	if got := uint32(cpu.Regs[3]); got != 0x00000080 {
		t.Fatalf("LBU got 0x%08X, want 0x00000080", got)
	}

	binary.BigEndian.PutUint32(bus.RAM[0x08:], 0x84040102) // LH r4,0x102(r0)
	if err := cpu.Step(); err != nil {
		t.Fatalf("LH step returned error: %v", err)
	}
	if got := uint32(cpu.Regs[4]); got != 0xFFFF8001 {
		t.Fatalf("LH got 0x%08X, want 0xFFFF8001", got)
	}

	binary.BigEndian.PutUint32(bus.RAM[0x0C:], 0x94050102) // LHU r5,0x102(r0)
	if err := cpu.Step(); err != nil {
		t.Fatalf("LHU step returned error: %v", err)
	}
	if got := uint32(cpu.Regs[5]); got != 0x00008001 {
		t.Fatalf("LHU got 0x%08X, want 0x00008001", got)
	}
}

func TestDoublewordLoadStoreUseBigEndianWordOrder(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	binary.BigEndian.PutUint32(bus.RAM[0x100:], 0x11223344)
	binary.BigEndian.PutUint32(bus.RAM[0x104:], 0x55667788)
	cpu.PC = 0x80000000
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0xDC020100) // LD r2,0x100(r0)

	if err := cpu.Step(); err != nil {
		t.Fatalf("LD step returned error: %v", err)
	}
	if cpu.Regs[2] != 0x1122334455667788 {
		t.Fatalf("LD loaded 0x%016X, want 0x1122334455667788", cpu.Regs[2])
	}

	cpu.Regs[3] = 0x99AABBCCDDEEFF00
	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0xFC030108) // SD r3,0x108(r0)
	if err := cpu.Step(); err != nil {
		t.Fatalf("SD step returned error: %v", err)
	}
	if got := binary.BigEndian.Uint64(bus.RAM[0x108:]); got != 0x99AABBCCDDEEFF00 {
		t.Fatalf("SD stored 0x%016X, want 0x99AABBCCDDEEFF00", got)
	}
}

func TestByteAndHalfwordMemoryOpsUseSignedOffsets(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80000000
	cpu.Regs[29] = 0x80000200
	cpu.Regs[2] = 0x000000AA
	cpu.Regs[3] = 0x00001234

	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0xA3A2FFFF) // SB r2,-1(sp)
	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0x93A4FFFF) // LBU r4,-1(sp)
	binary.BigEndian.PutUint32(bus.RAM[0x08:], 0xA7A3FFFC) // SH r3,-4(sp)
	binary.BigEndian.PutUint32(bus.RAM[0x0C:], 0x97A5FFFC) // LHU r5,-4(sp)

	for i := 0; i < 4; i++ {
		if err := cpu.Step(); err != nil {
			t.Fatalf("step %d returned error: %v", i, err)
		}
	}

	if got := bus.RAM[0x1FF]; got != 0xAA {
		t.Fatalf("SB -1(sp) wrote RAM[0x1FF]=0x%02X, want 0xAA", got)
	}
	if got := uint32(cpu.Regs[4]); got != 0xAA {
		t.Fatalf("LBU -1(sp) loaded 0x%08X, want 0x000000AA", got)
	}
	if got := binary.BigEndian.Uint16(bus.RAM[0x1FC:]); got != 0x1234 {
		t.Fatalf("SH -4(sp) wrote 0x%04X, want 0x1234", got)
	}
	if got := uint32(cpu.Regs[5]); got != 0x1234 {
		t.Fatalf("LHU -4(sp) loaded 0x%08X, want 0x00001234", got)
	}
}

func TestWritingPastCompareSetsTimerPending(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80000000
	cpu.COP0[Status] = 0
	cpu.COP0[Count] = 100
	cpu.Regs[4] = 90
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x40845800) // MTC0 a0, Compare

	if err := cpu.Step(); err != nil {
		t.Fatalf("MTC0 Compare step returned error: %v", err)
	}
	if cpu.COP0[Cause]&0x8000 == 0 {
		t.Fatalf("MTC0 Compare behind Count left timer IP7 clear: Count=0x%08X Compare=0x%08X Cause=0x%08X", cpu.COP0[Count], cpu.COP0[Compare], cpu.COP0[Cause])
	}
}

func TestInterruptExceptionSetsEXLNotERL(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80000000
	cpu.COP0[Status] = 0x00008001 // IE=1, IM7 enabled
	cpu.COP0[Cause] = 0x00008000  // IP7 pending
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x00000000)

	if err := cpu.Step(); err != nil {
		t.Fatalf("interrupt step returned error: %v", err)
	}
	if cpu.COP0[Status]&0x02 == 0 {
		t.Fatalf("interrupt did not set EXL: Status=0x%08X", cpu.COP0[Status])
	}
	if cpu.COP0[Status]&0x04 != 0 {
		t.Fatalf("interrupt set ERL instead of EXL: Status=0x%08X", cpu.COP0[Status])
	}
}

func TestBEVInterruptUsesGeneralExceptionVector(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80000000
	cpu.COP0[Status] = 0x00408001 // BEV=1, IE=1, IM7 enabled
	cpu.COP0[Cause] = 0x00008000  // IP7 pending
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x00000000)

	if err := cpu.Step(); err != nil {
		t.Fatalf("interrupt step returned error: %v", err)
	}
	if cpu.PC != 0xBFC00380 {
		t.Fatalf("BEV interrupt PC=0x%08X, want general exception vector 0xBFC00380", cpu.PC)
	}
}

func TestInterruptExceptionPreservesPendingCauseBits(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80000000
	cpu.COP0[Status] = 0x00408001 // BEV=1, IE=1, IM7 enabled
	cpu.COP0[Cause] = 0x00008000  // IP7 pending
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x00000000)

	if err := cpu.Step(); err != nil {
		t.Fatalf("interrupt step returned error: %v", err)
	}
	if cpu.COP0[Cause]&0x00008000 == 0 {
		t.Fatalf("interrupt exception cleared IP7 pending bit: Cause=0x%08X", cpu.COP0[Cause])
	}
}

func TestInterruptWithIVUsesSpecialInterruptVector(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80200000
	cpu.COP0[Status] = 0x00008001 // IE=1, IM7 enabled, BEV=0
	cpu.COP0[Cause] = 0x00808000  // IV=1, IP7 pending
	binary.BigEndian.PutUint32(bus.RAM[0x200000:], 0x00000000)

	if err := cpu.Step(); err != nil {
		t.Fatalf("interrupt step returned error: %v", err)
	}
	if cpu.PC != 0x80000200 {
		t.Fatalf("IV interrupt PC=0x%08X, want special interrupt vector 0x80000200", cpu.PC)
	}
	if cpu.COP0[Cause]&0x00800000 == 0 {
		t.Fatalf("interrupt exception cleared Cause.IV: Cause=0x%08X", cpu.COP0[Cause])
	}
}

func TestNestedTLBMissUsesGeneralExceptionVector(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80200000
	cpu.COP0[Status] = 0x00000002 // EXL=1, BEV=0
	cpu.RaiseTLBMiss(0x000DBB98)

	if cpu.PC != 0x80000180 {
		t.Fatalf("nested TLB miss PC=0x%08X, want general exception vector 0x80000180", cpu.PC)
	}
}

func TestTLBMissPreservesPendingCauseBits(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80200000
	cpu.COP0[Cause] = 0x00008000
	cpu.RaiseTLBMiss(0x00001234)

	if cpu.COP0[Cause]&0x00008000 == 0 {
		t.Fatalf("TLB miss cleared IP7 pending bit: Cause=0x%08X", cpu.COP0[Cause])
	}
}

func TestASICPendingInterruptMapsToHardwareIP2(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.COP0[Status] = 0x00000001 // IE=1, no IM bits enabled
	bus.Asic.BusIntStat = BUS_INT_RIO
	bus.Asic.BusIntEnable = BUS_INT_RIO

	cpu.checkInterrupts()

	if cpu.COP0[Cause]&0x0400 == 0 {
		t.Fatalf("ASIC pending interrupt did not set hardware IP2: Cause=0x%08X", cpu.COP0[Cause])
	}
	if cpu.COP0[Cause]&0x0100 != 0 {
		t.Fatalf("ASIC pending interrupt set software IP0: Cause=0x%08X", cpu.COP0[Cause])
	}
}

func TestStepPreservesASICBusInterruptCauseBit(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu

	cpu.PC = 0x80000000
	cpu.COP0[Status] = 0x00000002 // EXL=1
	cpu.COP0[Cause] = 0x00000400
	bus.Asic.BusIntStat = BUS_INT_RIO
	bus.Asic.BusIntEnable = BUS_INT_RIO
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x00000000)

	if err := cpu.Step(); err != nil {
		t.Fatalf("step returned error: %v", err)
	}
	if cpu.COP0[Cause]&0x00000400 == 0 {
		t.Fatalf("step cleared pending ASIC bus interrupt IP2: Cause=0x%08X", cpu.COP0[Cause])
	}
}

func TestTGEOnlyTrapsWhenConditionIsTrue(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu
	cpu.COP0[Status] = 0x00400000

	cpu.execSpecial(0x80000000, 0x00220030, 1, 2, 0, 0, ^uint64(1), ^uint64(0))
	if cpu.COP0[Cause]&0x7C != 0 {
		t.Fatalf("false TGE raised exception: Cause=0x%08X", cpu.COP0[Cause])
	}

	cpu.execSpecial(0x80000004, 0x00220030, 1, 2, 0, 0, ^uint64(0), ^uint64(1))
	if got := (cpu.COP0[Cause] >> 2) & 0x1F; got != 13 {
		t.Fatalf("true TGE exception code=%d, want 13", got)
	}
	if cpu.COP0[EPC] != 0x80000004 || cpu.PC != 0xBFC00380 {
		t.Fatalf("true TGE EPC=0x%08X PC=0x%08X", cpu.COP0[EPC], cpu.PC)
	}
}

func TestTEQIsTrapNotJump(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	cpu.COP0[Status] = 0x00400000
	cpu.execSpecial(0x80000000, 0x00220034, 1, 2, 0, 0, 7, 7)
	if got := (cpu.COP0[Cause] >> 2) & 0x1F; got != 13 {
		t.Fatalf("TEQ exception code=%d, want 13", got)
	}
	if cpu.DelaySlot {
		t.Fatal("TEQ incorrectly scheduled a branch")
	}
}

func TestDMULTProducesSigned128BitProduct(t *testing.T) {
	cpu := NewCPU(NewBus())
	cpu.execSpecial(0, 0x0000001C, 0, 0, 0, 0, ^uint64(1), 3)
	if cpu.HI != 0xFFFFFFFFFFFFFFFF || cpu.LO != 0xFFFFFFFFFFFFFFFA {
		t.Fatalf("DMULT HI:LO=%016X:%016X", cpu.HI, cpu.LO)
	}
}

func TestDMULTUProducesUnsigned128BitProduct(t *testing.T) {
	cpu := NewCPU(NewBus())
	cpu.execSpecial(0, 0x0000001D, 0, 0, 0, 0, 0xFFFFFFFFFFFFFFFF, 2)
	if cpu.HI != 1 || cpu.LO != 0xFFFFFFFFFFFFFFFE {
		t.Fatalf("DMULTU HI:LO=%016X:%016X", cpu.HI, cpu.LO)
	}
}

func TestREGIMMBranchLikelyNotTakenAnnulsDelaySlot(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	bus.CPU = cpu
	cpu.PC = 0x80000000
	cpu.Regs[8] = 1
	binary.BigEndian.PutUint32(bus.RAM[0x00:], 0x05020001) // BLTZL t0,+1
	binary.BigEndian.PutUint32(bus.RAM[0x04:], 0x24840001) // ADDIU a0,a0,1

	if err := cpu.Step(); err != nil {
		t.Fatalf("BLTZL step returned error: %v", err)
	}
	if cpu.PC != 0x80000008 {
		t.Fatalf("BLTZL not-taken PC=0x%08X, want 0x80000008", cpu.PC)
	}
	if err := cpu.Step(); err != nil {
		t.Fatalf("post-BLTZL step returned error: %v", err)
	}
	if cpu.Regs[4] != 0 {
		t.Fatalf("BLTZL not-taken executed delay slot, a0=%d", cpu.Regs[4])
	}
}

func TestREGIMMImmediateTrap(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	cpu.COP0[Status] = 0x00400000
	cpu.execRegImm(0x80000000, 1, 0x0C, 7, 7)
	if got := (cpu.COP0[Cause] >> 2) & 0x1F; got != 13 {
		t.Fatalf("TEQI exception code=%d, want 13", got)
	}
}
