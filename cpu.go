package main

import (
	"fmt"
	"math/bits"
)

const (
	Index    = 0
	Random   = 1
	EntryLo0 = 2
	EntryLo1 = 3
	Context  = 4
	PageMask = 5
	Wired    = 6
	BadVAddr = 8
	Count    = 9
	EntryHi  = 10
	Compare  = 11
	Status   = 12
	Cause    = 13
	EPC      = 14
	PRId     = 15
	Config   = 16
	LLAddr   = 17
	WatchLo  = 18
	WatchHi  = 19
	XContext = 20
	TagLo    = 28
	TagHi    = 29
	ErrorEPC = 30
)

type TLBEntry struct {
	Mask uint32
	Hi   uint32
	Lo0  uint32
	Lo1  uint32
}

const NumTLB = 48

type CPU struct {
	Regs [32]uint64
	HI   uint64
	LO   uint64
	PC   uint32

	COP0 [32]uint32
	TLB  [NumTLB]TLBEntry

	Bus             *Bus
	DelaySlot       bool
	DelayPC         uint32
	DelayBranchPC   uint32
	LLBit           bool
	tlbMiss         bool
	lastTLBMissPC   uint32
	lastTLBMissAddr uint32
	excPC           uint32
	excInDelaySlot  bool

	Steps uint64
}

func NewCPU(bus *Bus) *CPU {
	c := &CPU{Bus: bus}
	c.COP0[PRId] = 0x00002200   // R4640
	c.COP0[Status] = 0x00400006 // BEV=1, EXL=1, ERL=1
	c.COP0[Config] = 0x0080C683
	return c
}

func (c *CPU) Reset() {
	for i := range c.Regs {
		c.Regs[i] = 0
	}
	c.PC = 0xBFC00000
	c.HI, c.LO = 0, 0
	c.DelaySlot = false
	c.DelayPC = 0
	c.DelayBranchPC = 0
	c.excInDelaySlot = false
	c.LLBit = false
	c.COP0[Status] = 0x00400006 // BEV=1, EXL=1, ERL=1
	c.COP0[Cause] = 0
	c.COP0[EPC] = 0
	c.COP0[Count] = 0
	c.COP0[Compare] = 0
	c.COP0[Random] = uint32(NumTLB - 1)
	c.COP0[Wired] = 0
	c.Regs[29] = 0x807FFF00 // sp — top of 8MB RAM
	c.Regs[4] = 0x807FFF00  // a0 — boot arg (stack ptr on real hw)
}

func sign16(v uint16) int32 { return int32(int16(v)) }
func sign8(v uint8) int32   { return int32(int8(v)) }

func (c *CPU) Translate(vaddr uint32) (uint32, bool) {
	if vaddr >= 0x80000000 && vaddr < 0xC0000000 {
		return vaddr & 0x1FFFFFFF, true
	}
	if vaddr >= 0xC0000000 && vaddr < 0xC0000000+RAM_SIZE {
		return vaddr & 0x1FFFFFFF, true
	}
	asid := c.COP0[EntryHi] & 0xFF
	for i := 0; i < NumTLB; i++ {
		e := &c.TLB[i]
		mask := e.Mask | 0x1FFF
		vpn2mask := ^mask & 0xFFFFE000
		if (vaddr & vpn2mask) != (e.Hi & vpn2mask) {
			continue
		}
		g := (e.Lo0&1) != 0 && (e.Lo1&1) != 0
		if !g && (e.Hi&0xFF) != asid {
			continue
		}
		half := (mask + 1) >> 1
		lo := e.Lo0
		if (vaddr & half) != 0 {
			lo = e.Lo1
		}
		if (lo & 2) == 0 {
			return 0, false
		}
		pfn := (lo >> 6) & 0xFFFFF
		return (pfn << 12) | (vaddr & (half - 1)), true
	}
	return 0, false
}

func (c *CPU) setReg(r uint32, v uint64) {
	if r != 0 {
		c.Regs[r] = v
	}
}

func (c *CPU) setReg32(r uint32, v uint32) {
	if r != 0 {
		c.Regs[r] = uint64(int64(int32(v)))
	}
}

func (c *CPU) execLWL(rt uint32, addr uint32) {
	aligned := addr &^ 3
	offset := addr & 3
	old := uint32(c.Regs[rt])
	word := c.Bus.Read32(aligned)
	if c.tlbMiss {
		return
	}
	mask := uint32(0)
	if offset != 0 {
		mask = (uint32(1) << (offset * 8)) - 1
	}
	c.setReg32(rt, (old&mask)|(word<<(offset*8)))
}

func (c *CPU) execLWR(rt uint32, addr uint32) {
	aligned := addr &^ 3
	offset := addr & 3
	old := uint32(c.Regs[rt])
	word := c.Bus.Read32(aligned)
	if c.tlbMiss {
		return
	}
	mask := uint32(0)
	if offset != 3 {
		mask = ^((uint32(1) << ((offset + 1) * 8)) - 1)
	}
	c.setReg32(rt, (old&mask)|(word>>((3-offset)*8)))
}

func (c *CPU) execSWL(rt uint32, addr uint32) {
	val := uint32(c.Regs[rt])
	offset := addr & 3
	for i := uint32(0); i < 4-offset; i++ {
		c.Bus.Write8(addr+i, (val>>(24-i*8))&0xFF)
		if c.tlbMiss {
			return
		}
	}
}

func (c *CPU) execSWR(rt uint32, addr uint32) {
	val := uint32(c.Regs[rt])
	offset := addr & 3
	for i := uint32(0); i <= offset; i++ {
		c.Bus.Write8((addr&^3)+i, (val>>((offset-i)*8))&0xFF)
		if c.tlbMiss {
			return
		}
	}
}

func (c *CPU) Step() error {
	c.Steps++
	if c.Bus.SkipNullCleanup && c.PC == 0x802E80C4 && c.Bus.Read32(uint32(c.Regs[16])+0x110) == 0 {
		fmt.Printf("skipped null cleanup step=%d object=0x%08X ra=0x%08X\n", c.Steps, uint32(c.Regs[16]), uint32(c.Regs[31]))
		c.PC = uint32(c.Regs[31])
		c.DelaySlot = false
		return nil
	}
	if c.COP0[Random] > c.COP0[Wired] {
		c.COP0[Random]--
	} else {
		c.COP0[Random] = uint32(NumTLB - 1)
	}
	c.Bus.Asic.Tick(1)

	// COP0 Count increments every other cycle (each Step = 1 cycle)
	if c.Steps&1 == 0 {
		c.COP0[Count]++
		if c.COP0[Compare] != 0 && c.COP0[Count] >= c.COP0[Compare] {
			c.COP0[Cause] |= 0x8000 // set IP7 (timer interrupt pending)
		}
	}

	if c.Bus.Asic.PendingIP2() {
		c.COP0[Cause] |= 0x0400 // set IP2
	} else {
		c.COP0[Cause] &^= 0x0400 // clear IP2
	}

	pc := c.PC
	c.excInDelaySlot = c.DelaySlot
	if c.excInDelaySlot {
		c.excPC = c.DelayBranchPC
	} else {
		c.excPC = pc
	}
	c.tlbMiss = false

	if c.checkInterrupts() {
		return nil
	}

	inst := c.Bus.Read32(pc)

	if c.tlbMiss {
		return nil
	}
	if c.Bus.TraceNullFlow && pc >= 0x802E8080 && pc < 0x802E8100 && c.Bus.TraceNullCount < 64 {
		fmt.Printf("null-flow step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X s0=0x%08X s1=0x%08X s2=0x%08X s3=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X v0=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[16]), uint32(c.Regs[17]), uint32(c.Regs[18]), uint32(c.Regs[19]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[6]), uint32(c.Regs[2]), uint32(c.Regs[31]))
		c.Bus.TraceNullCount++
	}
	if c.Bus.TraceDeviceField && pc >= 0x8026D6C0 && pc < 0x8026D7DC && c.Bus.TraceNullCount < 4096 {
		fmt.Printf("resource-flow step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X v0=0x%08X v1=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X s0=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[2]), uint32(c.Regs[3]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[6]), uint32(c.Regs[16]), uint32(c.Regs[31]))
		c.Bus.TraceNullCount++
	}
	if c.Bus.TraceDeviceField && c.Steps >= 45_600_000 && pc >= 0x8026D24C && pc < 0x8026D3C0 && c.Bus.TraceFactoryCount < 256 {
		fmt.Printf("factory-flow step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X v0=0x%08X v1=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X s0=0x%08X s1=0x%08X s2=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[2]), uint32(c.Regs[3]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[6]), uint32(c.Regs[16]), uint32(c.Regs[17]), uint32(c.Regs[18]), uint32(c.Regs[31]))
		c.Bus.TraceFactoryCount++
	}
	if c.Bus.TraceDeviceField && pc >= 0x8026C288 && pc < 0x8026C318 && c.Bus.TraceRegistryInitCount < 64 {
		fmt.Printf("registry-init step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X v0=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[2]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[16]), uint32(c.Regs[17]), uint32(c.Regs[31]))
		c.Bus.TraceRegistryInitCount++
	}
	if c.Bus.TraceDeviceField && pc == 0x802E8D0C {
		fmt.Printf("factory-manager-init step=%d a0=0x%08X ra=0x%08X\n", c.Steps, uint32(c.Regs[4]), uint32(c.Regs[31]))
	}
	if c.Bus.TraceDeviceField && pc >= 0x802FBB40 && pc < 0x802FBBC0 && c.Bus.TraceFatalCount < 64 {
		fmt.Printf("fatal-callback step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X v0=0x%08X v1=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X s0=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[2]), uint32(c.Regs[3]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[6]), uint32(c.Regs[16]), uint32(c.Regs[31]))
		c.Bus.TraceFatalCount++
	}
	if c.Bus.TraceDeviceField && pc >= 0x802FB5E0 && pc < 0x802FB640 && c.Bus.TraceCallbackPrepCount < 64 {
		fmt.Printf("callback-prep step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X v0=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X a3=0x%08X s0=0x%08X sp=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[2]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[6]), uint32(c.Regs[7]), uint32(c.Regs[16]), uint32(c.Regs[29]), uint32(c.Regs[31]))
		c.Bus.TraceCallbackPrepCount++
	}
	if c.Bus.TraceDeviceField && pc >= 0x802FB500 && pc < 0x802FB640 && uint32(c.Regs[5]) == 0x8034845C && c.Bus.TraceCallbackOriginCount < 96 {
		fmt.Printf("callback-origin step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X v0=0x%08X v1=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X a3=0x%08X s0=0x%08X s1=0x%08X sp=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[2]), uint32(c.Regs[3]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[6]), uint32(c.Regs[7]), uint32(c.Regs[16]), uint32(c.Regs[17]), uint32(c.Regs[29]), uint32(c.Regs[31]))
		c.Bus.TraceCallbackOriginCount++
	}
	if c.Bus.TraceDeviceField && pc >= 0x802FC680 && pc < 0x802FC780 && c.Steps >= 38_700_000 && c.Steps < 38_800_000 && c.Bus.TraceTimerEventCount < 96 {
		fmt.Printf("timer-event step=%d pc=0x%08X inst=0x%08X delay=%t target=0x%08X v0=0x%08X v1=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X a3=0x%08X s0=0x%08X s1=0x%08X sp=0x%08X ra=0x%08X\n", c.Steps, pc, inst, c.DelaySlot, c.DelayPC, uint32(c.Regs[2]), uint32(c.Regs[3]), uint32(c.Regs[4]), uint32(c.Regs[5]), uint32(c.Regs[6]), uint32(c.Regs[7]), uint32(c.Regs[16]), uint32(c.Regs[17]), uint32(c.Regs[29]), uint32(c.Regs[31]))
		c.Bus.TraceTimerEventCount++
	}

	if c.DelaySlot {
		c.PC = c.DelayPC
		c.DelaySlot = false
	} else {
		c.PC = pc + 4
	}

	op := (inst >> 26) & 0x3F
	rs := (inst >> 21) & 0x1F
	rt := (inst >> 16) & 0x1F
	rd := (inst >> 11) & 0x1F
	shamt := (inst >> 6) & 0x1F
	imm := inst & 0xFFFF
	jimm := inst & 0x03FFFFFF

	rsv := c.Regs[rs]
	rtv := c.Regs[rt]
	simmediate := sign16(uint16(imm))

	switch op {
	case 0x00:
		c.execSpecial(pc, inst, rs, rt, rd, shamt, rsv, rtv)
	case 0x01:
		c.execRegImm(pc, rs, rt, rsv, simmediate)
	case 0x02:
		c.branch(pc, (pc&0xF0000000)|(jimm<<2))
	case 0x03:
		target := (pc & 0xF0000000) | (jimm << 2)
		c.setReg32(31, pc+8)
		c.branch(pc, target)
	case 0x04:
		if rsv == rtv {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		}
	case 0x05:
		if rsv != rtv {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		}
	case 0x06:
		if int64(rsv) <= 0 {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		}
	case 0x07:
		if int64(rsv) > 0 {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		}
	case 0x14: // BEQL
		if rsv == rtv {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		} else {
			c.PC = pc + 8
		}
	case 0x15: // BNEL
		if rsv != rtv {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		} else {
			c.PC = pc + 8
		}
	case 0x16: // BLEZL
		if int64(rsv) <= 0 {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		} else {
			c.PC = pc + 8
		}
	case 0x17: // BGTZL
		if int64(rsv) > 0 {
			c.branch(pc, uint32(int32(pc)+4+simmediate*4))
		} else {
			c.PC = pc + 8
		}
	case 0x08:
		c.setReg32(rt, uint32(rsv)+uint32(simmediate))
	case 0x09:
		c.setReg32(rt, uint32(rsv)+uint32(simmediate))
	case 0x0A:
		if int32(rsv) < simmediate {
			c.setReg32(rt, 1)
		} else {
			c.setReg32(rt, 0)
		}
	case 0x0B:
		if uint32(rsv) < uint32(simmediate) {
			c.setReg32(rt, 1)
		} else {
			c.setReg32(rt, 0)
		}
	case 0x0C:
		c.setReg32(rt, uint32(rsv)&uint32(imm))
	case 0x0D:
		c.setReg32(rt, uint32(rsv)|uint32(imm))
	case 0x0E:
		c.setReg32(rt, uint32(rsv)^uint32(imm))
	case 0x0F:
		c.setReg32(rt, uint32(imm)<<16)
	case 0x10:
		c.execCOP0(rs, rt, rd, inst)
	case 0x11, 0x12: // COP1/COP2 — NOP stubs (no FPU)
	case 0x18:
		c.setReg(rt, rsv+uint64(int64(simmediate)))
	case 0x19:
		c.setReg(rt, rsv+uint64(uint64(int32(simmediate))))
	case 0x1C:
		c.execSpecial2(inst, rs, rt, rd, rsv, rtv)
	case 0x20: // LB
		val := c.Bus.Read8(uint32(rsv) + uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
		c.setReg32(rt, uint32(sign8(val)))
	case 0x21: // LH
		val := c.Bus.Read16(uint32(rsv) + uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
		c.setReg32(rt, uint32(sign16(val)))
	case 0x22: // LWL
		c.execLWL(rt, uint32(rsv)+uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
	case 0x23: // LW
		addr := uint32(rsv) + uint32(simmediate)
		if addr&3 != 0 {
			c.exception(addr, 0x04)
			return nil
		}
		val := c.Bus.Read32(addr)
		if c.tlbMiss {
			return nil
		}
		c.setReg32(rt, val)
	case 0x24: // LBU
		val := c.Bus.Read8(uint32(rsv) + uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
		c.setReg32(rt, uint32(val))
	case 0x25: // LHU
		val := c.Bus.Read16(uint32(rsv) + uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
		c.setReg32(rt, uint32(val))
	case 0x26: // LWR
		c.execLWR(rt, uint32(rsv)+uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
	case 0x28: // SB
		c.Bus.Write8(uint32(rsv)+uint32(simmediate), uint32(rtv))
		if c.tlbMiss {
			return nil
		}
	case 0x29: // SH
		c.Bus.Write16(uint32(rsv)+uint32(simmediate), uint32(rtv))
		if c.tlbMiss {
			return nil
		}
	case 0x2A: // SWL
		c.execSWL(rt, uint32(rsv)+uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
	case 0x2B: // SW
		addr := uint32(rsv) + uint32(simmediate)
		if addr&3 != 0 {
			c.exception(addr, 0x05)
			return nil
		}
		c.Bus.Write32(addr, uint32(rtv))
		if c.tlbMiss {
			return nil
		}
	case 0x2E: // SWR
		c.execSWR(rt, uint32(rsv)+uint32(simmediate))
		if c.tlbMiss {
			return nil
		}
	case 0x38: // SC
		if c.LLBit {
			addr := uint32(rsv) + uint32(simmediate)
			if addr&3 != 0 {
				c.exception(addr, 0x05)
				return nil
			}
			c.Bus.Write32(addr, uint32(rtv))
			if c.tlbMiss {
				return nil
			}
			c.setReg32(rt, 1)
		} else {
			c.setReg32(rt, 0)
		}
	case 0x2F: // SDC1 — NOP stub (no FPU)
	case 0x31: // LWC1 — NOP stub (no FPU)
	case 0x35: // LDC1 — NOP stub (no FPU)
	case 0x37: // LD
		addr := uint32(rsv) + uint32(simmediate)
		hi := c.Bus.Read32(addr)
		if c.tlbMiss {
			return nil
		}
		lo := c.Bus.Read32(addr + 4)
		if c.tlbMiss {
			return nil
		}
		c.setReg(rt, uint64(lo)|(uint64(hi)<<32))
	case 0x39: // SWC1 — NOP stub (no FPU)
	case 0x3D: // SDC1 — NOP stub (no FPU)
	case 0x3B: // SW (duplicate)
		c.Bus.Write32(uint32(rsv)+uint32(simmediate), uint32(rtv))
		if c.tlbMiss {
			return nil
		}
	case 0x3F: // SD
		addr := uint32(rsv) + uint32(simmediate)
		c.Bus.Write32(addr, uint32(rtv>>32))
		if c.tlbMiss {
			return nil
		}
		c.Bus.Write32(addr+4, uint32(rtv))
		if c.tlbMiss {
			return nil
		}
	default:
		return fmt.Errorf("unhandled opcode 0x%02X at PC=0x%08X inst=0x%08X EPC=0x%08X BadVAddr=0x%08X Status=0x%08X Cause=0x%08X lastTLBPC=0x%08X lastTLBAddr=0x%08X", op, pc, inst, c.COP0[EPC], c.COP0[BadVAddr], c.COP0[Status], c.COP0[Cause], c.lastTLBMissPC, c.lastTLBMissAddr)
	}

	c.Regs[0] = 0
	return nil
}

func (c *CPU) execSpecial(pc, inst, rs, rt, rd, shamt uint32, rsv, rtv uint64) {
	funct := inst & 0x3F

	switch funct {
	// 32-bit shifts
	case 0x00: // SLL
		c.setReg32(rd, uint32(rtv)<<shamt)
	case 0x02: // SRL
		c.setReg32(rd, uint32(rtv)>>shamt)
	case 0x03: // SRA
		c.setReg32(rd, uint32(int32(rtv)>>shamt))
	case 0x04: // SLLV
		c.setReg32(rd, uint32(rtv)<<uint32(rsv&0x1F))
	case 0x06: // SRLV
		c.setReg32(rd, uint32(rtv)>>uint32(rsv&0x1F))
	case 0x07: // SRAV
		c.setReg32(rd, uint32(int32(rtv)>>uint32(rsv&0x1F)))

	// Jumps/traps
	case 0x08: // JR
		c.branch(pc, uint32(rsv))
	case 0x09: // JALR
		c.setReg32(rd, pc+8)
		c.branch(pc, uint32(rsv))
		return
	case 0x0A: // MOVZ
		if rtv == 0 {
			c.setReg(rd, rsv)
		}
	case 0x0B: // MOVN
		if rtv != 0 {
			c.setReg(rd, rsv)
		}
	case 0x0C: // SYSCALL
		c.exception(pc, 0x08)
		return
	case 0x0D: // BREAK
		c.setReg32(rd, pc+8)
		c.exception(pc, 0x09)
		return

	// HI/LO moves
	case 0x10: // MFHI
		c.setReg(rd, c.HI)
	case 0x11: // MTHI
		c.HI = rsv
	case 0x12: // MFLO
		c.setReg(rd, c.LO)
	case 0x13: // MTLO
		c.LO = rsv

	// Variable 64-bit shifts
	case 0x14: // DSLLV
		c.setReg(rd, rtv<<uint32(rsv&0x3F))
	case 0x16: // DSRLV
		c.setReg(rd, rtv>>uint32(rsv&0x3F))
	case 0x17: // DSRAV
		c.setReg(rd, uint64(int64(rtv)>>uint32(rsv&0x3F)))

	// 32-bit multiply/divide
	case 0x18: // MULT
		result := int64(int32(rsv)) * int64(int32(rtv))
		c.LO = uint64(int32(result))
		c.HI = uint64(int32(result >> 32))
	case 0x19: // MULTU
		result := uint64(uint32(rsv)) * uint64(uint32(rtv))
		c.LO = uint64(int32(result))
		c.HI = uint64(int32(result >> 32))
	case 0x1A: // DIV
		if rtv != 0 {
			c.LO = uint64(int32(int32(rsv) / int32(rtv)))
			c.HI = uint64(int32(int32(rsv) % int32(rtv)))
		}
	case 0x1B: // DIVU
		if rtv != 0 {
			c.LO = uint64(int32(uint32(rsv) / uint32(rtv)))
			c.HI = uint64(int32(uint32(rsv) % uint32(rtv)))
		}

	// 64-bit multiply/divide
	case 0x1C: // DMULT
		hi, lo := bits.Mul64(rsv, rtv)
		if int64(rsv) < 0 {
			hi -= rtv
		}
		if int64(rtv) < 0 {
			hi -= rsv
		}
		c.LO = lo
		c.HI = hi
	case 0x1D: // DMULTU
		c.HI, c.LO = bits.Mul64(rsv, rtv)
	case 0x1E: // DDIV
		if rtv != 0 {
			c.LO = uint64(int64(rsv) / int64(rtv))
			c.HI = uint64(int64(rsv) % int64(rtv))
		}
	case 0x1F: // DDIVU
		if rtv != 0 {
			c.LO = rsv / rtv
			c.HI = rsv % rtv
		}

	// 32-bit ALU
	case 0x20: // ADD
		c.setReg32(rd, uint32(rsv)+uint32(rtv))
	case 0x21: // ADDU
		c.setReg32(rd, uint32(rsv)+uint32(rtv))
	case 0x22: // SUB
		c.setReg32(rd, uint32(rsv)-uint32(rtv))
	case 0x23: // SUBU
		c.setReg32(rd, uint32(rsv)-uint32(rtv))
	case 0x24: // AND
		c.setReg(rd, rsv&rtv)
	case 0x25: // OR
		c.setReg(rd, rsv|rtv)
	case 0x26: // XOR
		c.setReg(rd, rsv^rtv)
	case 0x27: // NOR
		c.setReg(rd, ^(rsv | rtv))
	case 0x2A: // SLT
		if int32(rsv) < int32(rtv) {
			c.setReg32(rd, 1)
		} else {
			c.setReg32(rd, 0)
		}
	case 0x2B: // SLTU
		if rsv < rtv {
			c.setReg32(rd, 1)
		} else {
			c.setReg32(rd, 0)
		}

	// 64-bit ALU
	case 0x2C: // DADD
		c.setReg(rd, rsv+rtv)
	case 0x2D: // DADDU
		c.setReg(rd, rsv+rtv)
	case 0x2E: // DSUB
		c.setReg(rd, rsv-rtv)
	case 0x2F: // DSUBU
		c.setReg(rd, rsv-rtv)

	// 64-bit shifts
	case 0x38: // DSLL
		c.setReg(rd, rtv<<shamt)
	case 0x3A: // DSRL
		c.setReg(rd, rtv>>shamt)
	case 0x3B: // DSRA
		c.setReg(rd, uint64(int64(rtv)>>shamt))
	case 0x3C: // DSLL32
		c.setReg(rd, rtv<<(shamt+32))
	case 0x3E: // DSRL32
		c.setReg(rd, rtv>>(shamt+32))
	case 0x3F: // DSRA32
		c.setReg(rd, uint64(int64(rtv)>>(shamt+32)))

	// Traps
	case 0x30: // TGE
		if int64(rsv) >= int64(rtv) {
			c.exception(pc, 0x0D)
		}
	case 0x31: // TGEU
		if rsv >= rtv {
			c.exception(pc, 0x0D)
		}
	case 0x32: // TLT
		if int64(rsv) < int64(rtv) {
			c.exception(pc, 0x0D)
		}
	case 0x33: // TLTU
		if rsv < rtv {
			c.exception(pc, 0x0D)
		}
	case 0x34: // TEQ
		if rsv == rtv {
			c.exception(pc, 0x0D)
		}
	case 0x36: // TNE
		if rsv != rtv {
			c.exception(pc, 0x0D)
		}

	default:
		return // NOP/unknown
	}
}

func (c *CPU) execSpecial2(inst, rs, rt, rd uint32, rsv, rtv uint64) {
	funct := inst & 0x3F
	switch funct {
	case 0x00: // MADD
		result := int64(int32(c.LO)) + int64(int32(rsv))*int64(int32(rtv))
		c.LO = uint64(int32(result))
		c.HI = uint64(int32(result >> 32))
		c.setReg32(rd, uint32(c.LO))
	case 0x02: // MUL
		result := int64(int32(rsv)) * int64(int32(rtv))
		c.setReg32(rd, uint32(result))
	case 0x20: // DIVU (special2 variant)
		if rtv != 0 {
			c.LO = uint64(uint32(rsv) / uint32(rtv))
			c.HI = uint64(uint32(rsv) % uint32(rtv))
		}
		c.setReg32(rd, uint32(c.LO))
	default:
		c.setReg32(rd, 0)
	}
}

func (c *CPU) execRegImm(pc, rs, rt uint32, rsv uint64, simm int32) {
	target := uint32(int32(pc) + 4 + simm*4)
	switch rt {
	case 0x00: // BLTZ
		if int64(rsv) < 0 {
			c.branch(pc, target)
		}
	case 0x01: // BGEZ
		if int64(rsv) >= 0 {
			c.branch(pc, target)
		}
	case 0x02: // BLTZL
		if int64(rsv) < 0 {
			c.branch(pc, target)
		} else {
			c.PC = pc + 8
		}
	case 0x03: // BGEZL
		if int64(rsv) >= 0 {
			c.branch(pc, target)
		} else {
			c.PC = pc + 8
		}
	case 0x08: // TGEI
		if int64(rsv) >= int64(simm) {
			c.exception(pc, 0x0D)
		}
	case 0x09: // TGEIU
		if rsv >= uint64(int64(simm)) {
			c.exception(pc, 0x0D)
		}
	case 0x0A: // TLTI
		if int64(rsv) < int64(simm) {
			c.exception(pc, 0x0D)
		}
	case 0x0B: // TLTIU
		if rsv < uint64(int64(simm)) {
			c.exception(pc, 0x0D)
		}
	case 0x0C: // TEQI
		if int64(rsv) == int64(simm) {
			c.exception(pc, 0x0D)
		}
	case 0x0E: // TNEI
		if int64(rsv) != int64(simm) {
			c.exception(pc, 0x0D)
		}
	case 0x10: // BLTZAL
		c.setReg32(31, pc+8)
		if int64(rsv) < 0 {
			c.branch(pc, target)
		}
	case 0x11: // BGEZAL
		c.setReg32(31, pc+8)
		if int64(rsv) >= 0 {
			c.branch(pc, target)
		}
	case 0x12: // BLTZALL
		if int64(rsv) < 0 {
			c.setReg32(31, pc+8)
			c.branch(pc, target)
		} else {
			c.PC = pc + 8
		}
	case 0x13: // BGEZALL
		if int64(rsv) >= 0 {
			c.setReg32(31, pc+8)
			c.branch(pc, target)
		} else {
			c.PC = pc + 8
		}
	}
}

func (c *CPU) execCOP0(rs, rt, rd, inst uint32) {
	funct := inst & 0x3F
	switch rs {
	case 0x00: // MFC0
		c.setReg32(rt, c.COP0[rd])
	case 0x04: // MTC0
		if false && (rd == Status || rd == EntryHi || rd == EntryLo0 || rd == EntryLo1 || rd == PageMask || rd == Index) {
			fmt.Printf("MTC0 step=%d PC=0x%08X rd=%d old=0x%08X new=0x%08X Count=0x%08X Compare=0x%08X Cause=0x%08X Status=0x%08X a0=0x%08X ra=0x%08X\n",
				c.Steps, c.excPC, rd, c.COP0[rd], uint32(c.Regs[rt]), c.COP0[Count], c.COP0[Compare], c.COP0[Cause], c.COP0[Status], uint32(c.Regs[4]), uint32(c.Regs[31]))
		}
		c.COP0[rd] = uint32(c.Regs[rt])
		if rd == Status {
			c.checkInterrupts()
		}
		if rd == Compare {
			c.COP0[Cause] &^= 0x8000
			if c.COP0[Compare] != 0 && c.COP0[Count] >= c.COP0[Compare] {
				c.COP0[Cause] |= 0x8000
			}
		}
	case 0x10: // CO — TLB ops
		switch funct {
		case 0x02: // TLBWI
			idx := c.COP0[Index] & 0x3F
			if idx < NumTLB {
				c.TLB[idx] = TLBEntry{c.COP0[PageMask], c.COP0[EntryHi], c.COP0[EntryLo0], c.COP0[EntryLo1]}
				fmt.Printf("TLBWI step=%d PC=0x%08X idx=%d mask=0x%08X hi=0x%08X lo0=0x%08X lo1=0x%08X\n",
					c.Steps, c.excPC, idx, c.COP0[PageMask], c.COP0[EntryHi], c.COP0[EntryLo0], c.COP0[EntryLo1])
			}
		case 0x06: // TLBWR
			idx := c.COP0[Random] & 0x3F
			if idx < NumTLB {
				c.TLB[idx] = TLBEntry{c.COP0[PageMask], c.COP0[EntryHi], c.COP0[EntryLo0], c.COP0[EntryLo1]}
				fmt.Printf("TLBWR step=%d PC=0x%08X idx=%d mask=0x%08X hi=0x%08X lo0=0x%08X lo1=0x%08X\n",
					c.Steps, c.excPC, idx, c.COP0[PageMask], c.COP0[EntryHi], c.COP0[EntryLo0], c.COP0[EntryLo1])
			}
		case 0x01: // TLBR
			idx := c.COP0[Index] & 0x3F
			if idx < NumTLB {
				c.COP0[PageMask] = c.TLB[idx].Mask
				c.COP0[EntryHi] = c.TLB[idx].Hi
				c.COP0[EntryLo0] = c.TLB[idx].Lo0
				c.COP0[EntryLo1] = c.TLB[idx].Lo1
			}
		case 0x08: // TLBP
			asid := c.COP0[EntryHi] & 0xFF
			c.COP0[Index] &^= 0x8000003F
			c.COP0[Index] |= 0x80000000
			for i := 0; i < NumTLB; i++ {
				e := &c.TLB[i]
				mask := e.Mask | 0x1FFF
				vpn2mask := ^mask & 0xFFFFE000
				if (c.COP0[EntryHi] & vpn2mask) == (e.Hi & vpn2mask) {
					g := (e.Lo0&1) != 0 && (e.Lo1&1) != 0
					if g || (e.Hi&0xFF) == asid {
						c.COP0[Index] &^= 0x8000003F
						c.COP0[Index] |= uint32(i)
						break
					}
				}
			}
		case 0x18: // ERET
			pc := c.COP0[EPC]
			c.COP0[Status] &^= 0x02
			c.PC = pc
			return
		case 0x20: // WAIT
			c.waitForInterrupt()
		default:
		}
	case 0x18: // CO — ERET or TLB ops
		switch funct {
		case 0x18: // ERET
			pc := c.COP0[EPC]
			c.COP0[Status] &^= 0x02
			c.PC = pc
			return
		default:
		}
	}
}

func (c *CPU) waitForInterrupt() {
	cycles := uint64(0)
	if c.COP0[Compare] != 0 && c.COP0[Count] < c.COP0[Compare] {
		cycles = uint64(c.COP0[Compare]-c.COP0[Count]) * 2
	}
	if a := c.Bus.Asic; a != nil {
		if a.TimerCompare != 0 {
			target := uint64(a.TimerCompare) * 2
			if a.Cycles < target {
				delta := target - a.Cycles
				if cycles == 0 || delta < cycles {
					cycles = delta
				}
			}
		}
		if a.TimIntEnable&(1<<2) != 0 {
			target := a.SysTimerNext
			if target == 0 && a.Cycles >= sysTimerStartCycles {
				target = a.Cycles + sysTimerPeriodCycles
			}
			if target != 0 && a.Cycles < target {
				delta := target - a.Cycles
				if cycles == 0 || delta < cycles {
					cycles = delta
				}
			}
		}
		if a.VIDIntEnable != 0 && a.VideoNext != 0 && a.Cycles < a.VideoNext {
			delta := a.VideoNext - a.Cycles
			if cycles == 0 || delta < cycles {
				cycles = delta
			}
		}
	}
	if cycles != 0 {
		oldSteps := c.Steps
		c.Steps += cycles
		if oldSteps&1 != 0 {
			c.COP0[Count] += uint32((cycles + 1) / 2)
		} else {
			c.COP0[Count] += uint32(cycles / 2)
		}
		c.Bus.Asic.Tick(cycles)
	}
	if c.COP0[Compare] != 0 && c.COP0[Count] >= c.COP0[Compare] {
		c.COP0[Cause] |= 0x8000
	}
	if c.Bus.Asic.PendingIP2() {
		c.COP0[Cause] |= 0x0400
	}
}

func (c *CPU) RaiseTLBMiss(vaddr uint32) {
	if c.COP0[Status]&0x02 != 0 { // EXL already set — nested, don't redirect
	}
	nested := c.COP0[Status]&0x02 != 0
	c.tlbMiss = true
	c.lastTLBMissPC = c.excPC
	c.lastTLBMissAddr = vaddr
	c.COP0[BadVAddr] = vaddr
	vpn2 := vaddr & 0xFFFFE000
	c.COP0[EntryHi] = (c.COP0[EntryHi] & 0xFF) | vpn2
	c.COP0[Context] = (c.COP0[Context] & ^uint32(0xFFFC0000)) | ((vaddr >> 9) & 0xFFFC0000)
	code := uint32(2)
	bd := uint32(0)
	if c.excInDelaySlot {
		bd = 0x80000000
	}
	c.COP0[Cause] = (c.COP0[Cause] & 0x3080FF00) | bd | (code << 2)
	c.COP0[EPC] = c.excPC
	c.COP0[Status] |= 0x02
	bev := (c.COP0[Status] >> 22) & 1
	if bev != 0 {
		c.PC = 0xBFC00200
	} else if nested {
		c.PC = 0x80000180
	} else {
		c.PC = 0x80000000
	}
	c.DelaySlot = false
	c.excInDelaySlot = false
}

func (c *CPU) branch(pc, target uint32) {
	c.DelaySlot = true
	c.DelayPC = target
	c.DelayBranchPC = pc
}

func (c *CPU) exception(pc uint32, code uint32) {
	status := c.COP0[Status]
	if code == 0 && (status&0x00000001 == 0 || status&0x00000006 != 0) {
		return
	}
	if c.excInDelaySlot {
		pc = c.excPC
	}
	bd := uint32(0)
	if c.excInDelaySlot {
		bd = 0x80000000
	}
	c.COP0[Cause] = (c.COP0[Cause] & 0x3080FF00) | bd | ((code & 0x1F) << 2)
	c.COP0[EPC] = pc
	c.COP0[Status] = status | 0x02
	handler := uint32(0x80000180)
	if status&0x400000 != 0 {
		handler = 0xBFC00380
	} else if code == 0 && c.COP0[Cause]&0x00800000 != 0 {
		handler = 0x80000200
	}
	c.PC = handler
	c.DelaySlot = false
	c.excInDelaySlot = false
}

func (c *CPU) checkInterrupts() bool {
	status := c.COP0[Status]
	if status&0x00000001 == 0 || status&0x00000006 != 0 {
		return false
	}
	if c.Bus.Asic.PendingIP2() {
		c.COP0[Cause] |= 1 << 10 // IP2
	} else {
		c.COP0[Cause] &^= 1 << 10
	}
	cause := c.COP0[Cause]
	ip := (cause >> 8) & 0xFF
	im := (status >> 8) & 0xFF
	if ip&im != 0 {
		c.exception(c.excPC, 0x00)
		return true
	}
	return false
}
