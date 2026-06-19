package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const bootROMPath = "roms/bootrom.brom"

const forceEmptyModemScript = false
const forceSchedulerHeadReady = false
const forceBypassModemReadyWait = false

func main() {
	bus := NewBus()
	if err := bus.Asic.LoadNVRAM("roms/ptnvram.iic"); err != nil {
		fmt.Fprintf(os.Stderr, "NVRAM: %v\n", err)
	}
	cpu := NewCPU(bus)
	bus.CPU = cpu

	if err := loadBootROM(bus); err != nil {
		fmt.Fprintf(os.Stderr, "Boot ROM: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) >= 2 {
		ext := strings.ToLower(filepath.Ext(os.Args[1]))
		if ext == ".img" {
			if err := bus.IDE.Open(os.Args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "IDE: %v\n", err)
				os.Exit(1)
			}
			loadFirmwareToFlash(bus, os.Args[1])
		} else if len(os.Args) >= 3 {
			if err := loadFromChips(bus, os.Args[1], os.Args[2]); err != nil {
				fmt.Fprintf(os.Stderr, "Chips: %v\n", err)
				os.Exit(1)
			}
		}
	}

	cpu.Reset()
	cpu.PC = 0xBFC00000
	fmt.Printf("ROM_BASE=0x%08X ROM_MAX=%d len(ROM)=%d bus.Read32(0xBFC00000)=0x%08X ROM[0xC00000:4]=0x%02X%02X%02X%02X\n", ROM_BASE, ROM_MAX, len(bus.ROM), bus.Read32(0xBFC00000), bus.ROM[0xC00000], bus.ROM[0xC00001], bus.ROM[0xC00002], bus.ROM[0xC00003])
	fmt.Printf("WebTV Emulator — PC=0x%08X\n", cpu.PC)
	fmt.Printf("Running...\n")

	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "panic at PC=0x%08X: %v\n", cpu.PC, r)
			}
		}()

		handlerEntryHits := 0
		generalHandlerHits := 0
		var recent [8]struct {
			step int
			pc   uint32
			inst uint32
		}
		copyEntryHits := 0
		s4Changes := 0
		func302cHits := 0
		exceptionSaveHits := 0
		fatalLoopHits := 0
		lowGeneralHits := 0
		fwExceptionHits := 0
		nullLoopHits := 0
		idleLoopReported := false
		keyPollReported := false
		lateLoopReports := 0
		parserChars := 0
		parserFallbacks := 0
		parserErrors := 0
		lastResolverKey := ""
		bus.Tracking = true
		modemScriptEmptied := false
		modemReadyWaitPatched := false
		schedUnblocked := false
		for step := 0; step < 1_000_000_000; step++ {
			if step > 0 && step%100_000_000 == 0 {
				head := binary.BigEndian.Uint32(bus.RAM[0x18F0:])
				glob := binary.BigEndian.Uint32(bus.RAM[0x1900:])
				headTick, globTick := uint32(0), uint32(0)
				if hp := head & 0x1FFFFFFF; hp+0x13 < uint32(len(bus.RAM)) {
					headTick = binary.BigEndian.Uint32(bus.RAM[hp+0x10:])
				}
				if gp := glob & 0x1FFFFFFF; gp+0x13 < uint32(len(bus.RAM)) {
					globTick = binary.BigEndian.Uint32(bus.RAM[gp+0x10:])
				}
				fmt.Printf("[%dM] PC=0x%08X Status=0x%08X schedHead=0x%08X headTick=%d schedGlob=0x%08X globTick=%d\n", step/1_000_000, cpu.PC, cpu.COP0[Status], head, headTick, glob, globTick)
			}
			if forceEmptyModemScript && !modemScriptEmptied {
				ptr := binary.BigEndian.Uint32(bus.RAM[0x1C90:])
				if ptr >= 0x800F4C80 && ptr < 0x800F5500 {
					modemScriptEmptied = true
					base := uint32(0x000F4C80)
					for i := uint32(0); i < 0x900 && int(base+i) < len(bus.RAM); i++ {
						bus.RAM[base+i] = 0
					}
					fmt.Printf("modem script emptied step=%d ptr=0x%08X PC=0x%08X\n", step, ptr, cpu.PC)
				}
			}
			if forceBypassModemReadyWait && !modemReadyWaitPatched {
				const waitCond = uint32(0x000F50C6)
				if string(bus.RAM[waitCond:waitCond+12]) == "w(Idone\x00<C2\x00" {
					bus.RAM[waitCond+10] = '0'
					modemReadyWaitPatched = true
					fmt.Printf("modem ready wait patched step=%d addr=0x%08X PC=0x%08X\n", step, waitCond+10, cpu.PC)
				}
			}
			if forceSchedulerHeadReady && modemScriptEmptied && !schedUnblocked && step > 60_000_000 {
				head := binary.BigEndian.Uint32(bus.RAM[0x18F0:])
				glob := binary.BigEndian.Uint32(bus.RAM[0x1900:])
				hp := head & 0x1FFFFFFF
				gp := glob & 0x1FFFFFFF
				if hp+0x13 < uint32(len(bus.RAM)) && gp+0x13 < uint32(len(bus.RAM)) {
					headTick := binary.BigEndian.Uint32(bus.RAM[hp+0x10:])
					globTick := binary.BigEndian.Uint32(bus.RAM[gp+0x10:])
					if headTick != 0 && globTick != 0 && headTick < globTick {
						binary.BigEndian.PutUint32(bus.RAM[hp+0x10:], globTick)
						schedUnblocked = true
						fmt.Printf("scheduler head tick raised step=%d head=0x%08X %d->%d glob=0x%08X PC=0x%08X\n", step, head, headTick, globTick, glob, cpu.PC)
					}
				}
			}
			p := cpu.PC & 0x1FFFFFFF
			if p >= RAM_SIZE && (p < ROM_BASE || p >= ROM_BASE+uint32(ROM_MAX)) {
				fmt.Printf("PC left executable memory at step=%d PC=0x%08X phys=0x%08X Status=0x%08X\n", step, cpu.PC, p, cpu.COP0[Status])
				for i := 0; i < len(recent); i++ {
					entry := recent[(step+i)%len(recent)]
					if entry.step != 0 {
						fmt.Printf("  recent step=%d PC=0x%08X inst=0x%08X\n", entry.step, entry.pc, entry.inst)
					}
				}
				break
			}
			if cpu.PC == 0xBFC00200 {
				handlerEntryHits++
				if handlerEntryHits <= 5 {
					fmt.Printf("handler hit %d step=%d EPC=0x%08X BadVAddr=0x%08X Cause=0x%08X Status=0x%08X\n", handlerEntryHits, step, cpu.COP0[EPC], cpu.COP0[BadVAddr], cpu.COP0[Cause], cpu.COP0[Status])
				}
			}
			if cpu.PC == 0xBFC00380 {
				generalHandlerHits++
				if generalHandlerHits <= 8 {
					fmt.Printf("general handler hit %d step=%d EPC=0x%08X EPCInst=0x%08X Cause=0x%08X Status=0x%08X Count=0x%08X Compare=0x%08X\n",
						generalHandlerHits, step, cpu.COP0[EPC], bus.Read32(cpu.COP0[EPC]), cpu.COP0[Cause], cpu.COP0[Status], cpu.COP0[Count], cpu.COP0[Compare])
					for i := 0; i < len(recent); i++ {
						entry := recent[(step+i)%len(recent)]
						if entry.step != 0 {
							fmt.Printf("  pre-exception recent step=%d PC=0x%08X inst=0x%08X\n", entry.step, entry.pc, entry.inst)
						}
					}
					for r := 0; r < 32; r += 4 {
						fmt.Printf("  pre-exception r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X\n",
							r, cpu.Regs[r], r+1, cpu.Regs[r+1], r+2, cpu.Regs[r+2], r+3, cpu.Regs[r+3])
					}
				}
			}
			if cpu.PC == 0x9FC004DC {
				fmt.Printf("calibration divide step=%d s0=0x%016X s1=0x%016X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X\n",
					step, cpu.Regs[16], cpu.Regs[17], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5])
			}
			if cpu.PC == 0x802224B4 {
				exceptionSaveHits++
				if exceptionSaveHits <= 4 {
					fmt.Printf("exception save gp hit=%d step=%d k0=0x%016X k1=0x%016X gp=0x%016X sp=0x%016X BadVAddr=0x%08X Cause=0x%08X\n",
						exceptionSaveHits, step, cpu.Regs[26], cpu.Regs[27], cpu.Regs[28], cpu.Regs[29], cpu.COP0[BadVAddr], cpu.COP0[Cause])
				}
			}
			if cpu.PC == 0x80000180 {
				lowGeneralHits++
				if lowGeneralHits <= 4 {
					fmt.Printf("low general vector hit=%d step=%d EPC=0x%08X EPCInst=0x%08X Cause=0x%08X Status=0x%08X BadVAddr=0x%08X RA=0x%016X\n",
						lowGeneralHits, step, cpu.COP0[EPC], bus.Read32(cpu.COP0[EPC]), cpu.COP0[Cause], cpu.COP0[Status], cpu.COP0[BadVAddr], cpu.Regs[31])
				}
			}
			if cpu.PC == 0x802223B4 || cpu.PC == 0x80223120 {
				fwExceptionHits++
				if fwExceptionHits <= 8 {
					fmt.Printf("fw exception path hit=%d step=%d PC=0x%08X EPC=0x%08X Cause=0x%08X Status=0x%08X k0=0x%016X k1=0x%016X sp=0x%016X\n",
						fwExceptionHits, step, cpu.PC, cpu.COP0[EPC], cpu.COP0[Cause], cpu.COP0[Status], cpu.Regs[26], cpu.Regs[27], cpu.Regs[29])
				}
			}
			if cpu.PC == 0x8020936C || cpu.PC == 0x80209370 {
				fatalLoopHits++
				if fatalLoopHits == 1 {
					fmt.Printf("fatal loop entry step=%d PC=0x%08X EPC=0x%08X EPCInst=0x%08X Cause=0x%08X Status=0x%08X BadVAddr=0x%08X RA=0x%016X\n",
						step, cpu.PC, cpu.COP0[EPC], bus.Read32(cpu.COP0[EPC]), cpu.COP0[Cause], cpu.COP0[Status], cpu.COP0[BadVAddr], cpu.Regs[31])
					for r := 0; r < 32; r += 4 {
						fmt.Printf("  fatal r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X\n",
							r, cpu.Regs[r], r+1, cpu.Regs[r+1], r+2, cpu.Regs[r+2], r+3, cpu.Regs[r+3])
					}
					dumpRAMRange(bus, 0x00225C00, 0x200, "debug_80225c00.bin")
					dumpRAMRange(bus, 0x00209200, 0x100, "debug_80209200.bin")
					dumpRAMRange(bus, 0x00222E00, 0x200, "debug_80222e00.bin")
					dumpRAMRange(bus, 0x00229500, 0x120, "debug_80229500.bin")
				}
			}
			if cpu.PC == 0x802FBB64 || cpu.PC == 0x802FBB68 {
				nullLoopHits++
				if nullLoopHits <= 40 {
					owner := uint32(cpu.Regs[4]) & 0x1FFFFFFF
					base := uint32(0)
					count := uint32(0)
					if owner+0x1B < uint32(len(bus.RAM)) {
						base = binary.BigEndian.Uint32(bus.RAM[owner+0x14:])
						count = binary.BigEndian.Uint32(bus.RAM[owner+0x18:])
					}
					fmt.Printf("candidate loop hit=%d step=%d owner=0x%08X base=0x%08X count=%d s0/current=0x%016X s1/index=%d s2/key=%q\n",
						nullLoopHits, step, uint32(cpu.Regs[4]), base, count, cpu.Regs[16], cpu.Regs[17], previewRAMString(bus, uint32(cpu.Regs[18])&0x1FFFFFFF, 80))
				}
			}
			if cpu.PC == 0x8024B73C && !keyPollReported {
				keyPollReported = true
				ptr := binary.BigEndian.Uint32(bus.RAM[0x1C90:])
				b := uint8(0)
				if (ptr & 0x1FFFFFFF) < uint32(len(bus.RAM)) {
					b = bus.RAM[ptr&0x1FFFFFFF]
				}
				fmt.Printf("poll 0x8024B73C step=%d ptr=0x%08X byte0=0x%02X s1=0x%016X s2=0x%016X ra=0x%016X\n",
					step, ptr, b, cpu.Regs[17], cpu.Regs[18], cpu.Regs[31])
			}
			if cpu.PC == 0x8024A9D4 && parserChars < 96 {
				parserChars++
				ptr := binary.BigEndian.Uint32(bus.RAM[0x1C90:])
				ch := byte(cpu.Regs[2])
				fmt.Printf("parser char %02d step=%d ch=0x%02X %q ptr=0x%08X s0=0x%016X ra=0x%016X\n",
					parserChars, step, ch, string([]byte{ch}), ptr, cpu.Regs[16], cpu.Regs[31])
			}
			if cpu.PC == 0x8024AA60 && parserFallbacks < 8 {
				parserFallbacks++
				ptr := binary.BigEndian.Uint32(bus.RAM[0x1C90:])
				fmt.Printf("parser fallback %d step=%d v0=0x%02X ptr=0x%08X around=%q s0=0x%016X ra=0x%016X\n",
					parserFallbacks, step, byte(cpu.Regs[2]), ptr, previewRAMString(bus, (ptr-16)&0x1FFFFFFF, 96), cpu.Regs[16], cpu.Regs[31])
			}
			if (cpu.PC == 0x8024B900 || cpu.PC == 0x8024B9D8) && parserErrors < 64 {
				parserErrors++
				ptr := binary.BigEndian.Uint32(bus.RAM[0x1C90:])
				fmt.Printf("parser error call %02d step=%d PC=0x%08X code/a0=0x%016X val/a1=0x%016X ptr=0x%08X around=%q ra=0x%016X\n",
					parserErrors, step, cpu.PC, cpu.Regs[4], cpu.Regs[5], ptr, previewRAMString(bus, (ptr-24)&0x1FFFFFFF, 120), cpu.Regs[31])
			}
			if (cpu.PC == 0x9FC03040 || cpu.PC == 0xBFC03040) && cpu.Regs[5] >= 0x80200000 && cpu.Regs[5] < 0x80210000 && copyEntryHits < 8 {
				copyEntryHits++
				fmt.Printf("copy-to-entry %d step=%d PC=0x%08X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X ra=0x%016X\n", copyEntryHits, step, cpu.PC, cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[31])
			}
			if cpu.PC == 0x80221AB8 {
				fmt.Printf("Reached firmware BREAK at step=%d PC=0x%08X inst=0x%08X\n", step, cpu.PC, bus.Read32(cpu.PC))
				cfg334 := binary.BigEndian.Uint32(bus.RAM[0x334:])
				tableIndex := ((cfg334 >> 22) << 3) & 0xFFFF
				tableBase := uint32(0x003584E0)
				fmt.Printf("alloc config [0x334]=0x%08X tableIndex=0x%X table[base+idx]=0x%08X table[base+idx+4]=0x%08X\n",
					cfg334, tableIndex, binary.BigEndian.Uint32(bus.RAM[tableBase+tableIndex:]), binary.BigEndian.Uint32(bus.RAM[tableBase+tableIndex+4:]))
				dumpRAMRange(bus, 0x002C3000, 0x700, "debug_802c3000.bin")
				dumpRAMRange(bus, 0x002FBC00, 0x400, "debug_802fbc00.bin")
				dumpRAMRange(bus, 0x00211D00, 0x400, "debug_80211d00.bin")
				dumpRAMRange(bus, 0x00212100, 0x400, "debug_80212100.bin")
				for r := 0; r < 32; r += 4 {
					fmt.Printf("  r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X\n",
						r, cpu.Regs[r], r+1, cpu.Regs[r+1], r+2, cpu.Regs[r+2], r+3, cpu.Regs[r+3])
				}
				for i := 0; i < len(recent); i++ {
					entry := recent[(step+i)%len(recent)]
					if entry.step != 0 {
						fmt.Printf("  recent step=%d PC=0x%08X inst=0x%08X\n", entry.step, entry.pc, entry.inst)
					}
				}
				break
			}
			if cpu.PC == 0x802C30E4 && func302cHits < 24 {
				func302cHits++
				fmt.Printf("func 0x802C30E4 hit %d step=%d ra=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X sp=0x%016X\n",
					func302cHits, step, cpu.Regs[31], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[29])
			}
			if cpu.PC == 0x802FA414 {
				s3 := uint32(cpu.Regs[19])
				s4 := uint32(cpu.Regs[20])
				s0 := uint32(cpu.Regs[16])
				capacity := uint32(0)
				if (s3&0x1FFFFFFF)+0x13 < uint32(len(bus.RAM)) {
					capacity = binary.BigEndian.Uint32(bus.RAM[(s3&0x1FFFFFFF)+0x10:])
				}
				length := s0 - s4
				fmt.Printf("string bound check step=%d s2=0x%016X s3=0x%08X s4=0x%08X s0=0x%08X len=%d cap=%d bytes=%q\n",
					step, cpu.Regs[18], s3, s4, s0, length, capacity, previewRAMString(bus, s4&0x1FFFFFFF, 80))
			}
			if cpu.PC == 0x802FA1A0 || cpu.PC == 0x802FA2E8 || cpu.PC == 0x802FA3B4 || cpu.PC == 0x802FA438 {
				fmt.Printf("helper assert path step=%d PC=0x%08X s0=0x%016X s1=0x%016X s2=0x%016X s3=0x%016X s4=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X v0=0x%016X\n",
					step, cpu.PC, cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19], cpu.Regs[20], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[2])
				fmt.Printf("  assert strings a0=%q a1=%q fmtA=%q fmtB=%q key=%q\n",
					previewRAMString(bus, uint32(cpu.Regs[4])&0x1FFFFFFF, 120),
					previewRAMString(bus, uint32(cpu.Regs[5])&0x1FFFFFFF, 120),
					previewRAMString(bus, 0x003480F0, 120),
					previewRAMString(bus, 0x00348108, 120),
					previewRAMString(bus, uint32(cpu.Regs[16])&0x1FFFFFFF, 120))
				fmt.Printf("  suffix candidates 8120=%q 8124=%q 8128=%q 812C=%q 8130=%q\n",
					previewRAMString(bus, 0x00348120, 80),
					previewRAMString(bus, 0x00348124, 80),
					previewRAMString(bus, 0x00348128, 80),
					previewRAMString(bus, 0x0034812C, 80),
					previewRAMString(bus, 0x00348130, 80))
			}
			if cpu.PC == 0x802FA220 || cpu.PC == 0x802FA250 || cpu.PC == 0x802FA294 || cpu.PC == 0x802FA2C4 {
				bufOff := uint32(cpu.Regs[16]) & 0x1FFFFFFF
				spArg := binary.BigEndian.Uint32(bus.RAM[(uint32(cpu.Regs[29])&0x1FFFFFFF)+0x10:])
				before := previewRAMString(bus, bufOff, 120)
				shimLocalKey(bus, bufOff, spArg&0x1FFFFFFF)
				after := previewRAMString(bus, bufOff, 120)
				fmt.Printf("formatted key step=%d PC=0x%08X before=%q after=%q spArg=0x%08X\n",
					step, cpu.PC, before, after, spArg)
			}
			if cpu.PC == 0x802E6124 {
				lastResolverKey = previewRAMString(bus, uint32(cpu.Regs[4])&0x1FFFFFFF, 120)
				fmt.Printf("resolver hit step=%d ra=0x%016X key=%q global2574=0x%08X\n",
					step, cpu.Regs[31], lastResolverKey, binary.BigEndian.Uint32(bus.RAM[0x2574:]))
			}
			if cpu.PC == 0x802E6140 {
				bufOff := (uint32(cpu.Regs[29]) & 0x1FFFFFFF) + 0x10
				before := previewRAMString(bus, bufOff, 120)
				shimOptionsKey(bus, bufOff, lastResolverKey)
				fmt.Printf("resolver formatted step=%d fmt=%q input=%q generated=%q\n",
					step, previewRAMString(bus, 0x00345190, 120), lastResolverKey, previewRAMString(bus, bufOff, 120))
				if before != previewRAMString(bus, bufOff, 120) {
					fmt.Printf("  options shim before=%q after=%q\n", before, previewRAMString(bus, bufOff, 120))
				}
			}
			if cpu.PC == 0x802E61E0 || cpu.PC == 0x802E61F0 || cpu.PC == 0x802E6200 || cpu.PC == 0x802E6210 || cpu.PC == 0x802E6248 {
				fmt.Printf("resolver return step=%d PC=0x%08X v0=0x%016X keyArg/a0=%q f4=0x%08X fc=0x%08X f8=0x%08X global2574=0x%08X\n",
					step, cpu.PC, cpu.Regs[2], previewRAMString(bus, uint32(cpu.Regs[4])&0x1FFFFFFF, 120),
					binary.BigEndian.Uint32(bus.RAM[0x000EAD98:]), binary.BigEndian.Uint32(bus.RAM[0x000EADA0:]), binary.BigEndian.Uint32(bus.RAM[0x000EAD9C:]), binary.BigEndian.Uint32(bus.RAM[0x2574:]))
			}
			if cpu.PC >= 0x8022A430 && cpu.PC < 0x8022A500 && lateLoopReports < 40 {
				lateLoopReports++
				fmt.Printf("late loop %d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X t0=0x%016X t1=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					lateLoopReports, step, cpu.PC, bus.Read32(cpu.PC),
					cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[8], cpu.Regs[9], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
			}
			if cpu.PC == 0x8020912C && !idleLoopReported {
				idleLoopReported = true
				fmt.Printf("Reached 0x8020912C halt loop at step=%d Status=0x%08X Cause=0x%08X EPC=0x%08X BadVAddr=0x%08X RA=0x%016X\n",
					step, cpu.COP0[Status], cpu.COP0[Cause], cpu.COP0[EPC], cpu.COP0[BadVAddr], cpu.Regs[31])
				fmt.Printf("ASIC IRQ state BusStat=0x%08X BusEn=0x%08X TimStat=0x%08X TimEn=0x%08X TimerCompare=0x%08X ASICCycles=0x%016X ASICCount=0x%08X RIOStat=0x%08X RIOEn=0x%08X VIDStat=0x%08X VIDEn=0x%08X DevStat=0x%08X DevEn=0x%08X Count=0x%08X Compare=0x%08X\n",
					bus.Asic.BusIntStat, bus.Asic.BusIntEnable, bus.Asic.TimIntStat, bus.Asic.TimIntEnable,
					bus.Asic.TimerCompare, bus.Asic.Cycles, uint32(bus.Asic.Cycles>>1),
					bus.Asic.RIOIntStat, bus.Asic.RIOIntEnable, bus.Asic.VIDIntStat, bus.Asic.VIDIntEnable,
					bus.Asic.DevIntStat, bus.Asic.DevIntEnable, cpu.COP0[Count], cpu.COP0[Compare])
				dumpRAMRange(bus, 0x00209000, 0x200, "debug_80209000.bin")
				dumpRAMRange(bus, 0x00209300, 0x100, "debug_80209300.bin")
				dumpRAMRange(bus, 0x00211C00, 0x200, "debug_80211c00.bin")
				dumpRAMRange(bus, 0x00222000, 0x300, "debug_80222000.bin")
				dumpRAMRange(bus, 0x0022A500, 0x100, "debug_8022a500.bin")
				printRangeSummary(bus, 0x00400000, 0x00100000, "video-ish 0x80400000")
				printRangeSummary(bus, 0x00500000, 0x00100000, "firmware/heap 0x80500000")
				dumpRAMRange(bus, 0x00400000, 0x100000, "debug_video_80400000.bin")
				for i := 0; i < len(recent); i++ {
					entry := recent[(step+i)%len(recent)]
					if entry.step != 0 {
						fmt.Printf("  recent step=%d PC=0x%08X inst=0x%08X\n", entry.step, entry.pc, entry.inst)
					}
				}
			}
			recent[step%len(recent)] = struct {
				step int
				pc   uint32
				inst uint32
			}{step: step, pc: cpu.PC, inst: bus.Read32(cpu.PC)}
			beforeS4 := cpu.Regs[20]
			beforePC := cpu.PC
			beforeInst := recent[step%len(recent)].inst
			if err := cpu.Step(); err != nil {
				fmt.Fprintf(os.Stderr, "CPU error at PC=0x%08X: %v\n", cpu.PC, err)
				dumpRAMRange(bus, 0x002C7000, 0x400, "debug_802c7000.bin")
				dumpRAMRange(bus, 0x002FA100, 0x500, "debug_802fa100.bin")
				dumpRAMRange(bus, 0x00209500, 0x200, "debug_80209500.bin")
				dumpRAMRange(bus, 0x00223000, 0x200, "debug_80223000.bin")
				dumpRAMRange(bus, 0x00228000, 0x200, "debug_80228000.bin")
				dumpRAMRange(bus, 0x002E5200, 0x400, "debug_802e5200.bin")
				dumpRAMRange(bus, 0x002E6000, 0x500, "debug_802e6000.bin")
				dumpRAMRange(bus, 0x002E6300, 0x300, "debug_802e6300.bin")
				dumpRAMRange(bus, 0x002FBA00, 0x300, "debug_802fba00.bin")
				dumpRAMRange(bus, 0x002FDA00, 0x300, "debug_802fda00.bin")
				dumpRAMRange(bus, 0x002FDB00, 0x200, "debug_802fdb00.bin")
				fmt.Fprintf(os.Stderr, "  EPC=0x%08X BadVAddr=0x%08X Cause=0x%08X Status=0x%08X EntryHi=0x%08X Context=0x%08X\n",
					cpu.COP0[EPC], cpu.COP0[BadVAddr], cpu.COP0[Cause], cpu.COP0[Status], cpu.COP0[EntryHi], cpu.COP0[Context])
				for r := 0; r < 32; r += 4 {
					fmt.Fprintf(os.Stderr, "  r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X\n",
						r, cpu.Regs[r], r+1, cpu.Regs[r+1], r+2, cpu.Regs[r+2], r+3, cpu.Regs[r+3])
				}
				for i := 0; i < len(recent); i++ {
					entry := recent[(step+1+i)%len(recent)]
					if entry.step != 0 {
						fmt.Fprintf(os.Stderr, "  recent step=%d PC=0x%08X inst=0x%08X\n", entry.step, entry.pc, entry.inst)
					}
				}
				break
			}
			if cpu.Regs[20] != beforeS4 && beforePC >= 0x80200000 && s4Changes < 16 {
				s4Changes++
				fmt.Printf("s4 change %d step=%d PC=0x%08X inst=0x%08X old=0x%016X new=0x%016X\n",
					s4Changes, step, beforePC, beforeInst, beforeS4, cpu.Regs[20])
			}
		}
		fmt.Printf("Final instruction: PC=0x%08X inst=0x%08X EPC=0x%08X epcInst=0x%08X BadVAddr=0x%08X Cause=0x%08X\n",
			cpu.PC, bus.Read32(cpu.PC), cpu.COP0[EPC], bus.Read32(cpu.COP0[EPC]), cpu.COP0[BadVAddr], cpu.COP0[Cause])
		printRangeSummary(bus, 0x00400000, 0x00100000, "final video-ish 0x80400000")
		dumpRAMRange(bus, 0x00400000, 0x100000, "debug_video_80400000_final.bin")
		dumpRAMRange(bus, 0x002C7000, 0x2500, "debug_802c7000_full.bin")
		dumpRAMRange(bus, 0x00211000, 0x800, "debug_80211000.bin")
		dumpRAMRange(bus, 0x00001400, 0x500, "debug_irq_table.bin")
		dumpRAMRange(bus, 0x00212400, 0x300, "debug_80212400.bin")
		dumpRAMRange(bus, 0x00222200, 0x400, "debug_80222200.bin")
		dumpRAMRange(bus, 0x00221800, 0x500, "debug_80221800.bin")
		dumpRAMRange(bus, 0x00223600, 0x400, "debug_80223600.bin")
		dumpRAMRange(bus, 0x00000400, 0x400, "debug_lowram_0400.bin")
		if cpu.PC >= 0x80209000 && cpu.PC < 0x80209400 {
			dumpRAMRange(bus, 0x00209200, 0x100, "debug_80209200.bin")
			dumpRAMRange(bus, 0x00209300, 0x100, "debug_80209300.bin")
		}
		if cpu.PC >= 0x80222400 && cpu.PC < 0x80222600 {
			dumpRAMRange(bus, 0x00222300, 0x100, "debug_80222300.bin")
			dumpRAMRange(bus, 0x00222400, 0x200, "debug_80222400.bin")
		}
		if cpu.PC >= 0x8022A000 && cpu.PC < 0x8022B000 {
			for r := 0; r < 32; r += 4 {
				fmt.Printf("  finalA r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X\n",
					r, cpu.Regs[r], r+1, cpu.Regs[r+1], r+2, cpu.Regs[r+2], r+3, cpu.Regs[r+3])
			}
			dumpRAMRange(bus, 0x0022A300, 0x300, "debug_8022a300.bin")
		}
		if cpu.PC >= 0x8024A000 && cpu.PC < 0x8024C000 {
			for r := 0; r < 32; r += 4 {
				fmt.Printf("  final r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X\n",
					r, cpu.Regs[r], r+1, cpu.Regs[r+1], r+2, cpu.Regs[r+2], r+3, cpu.Regs[r+3])
			}
			ptr := binary.BigEndian.Uint32(bus.RAM[0x1C90:])
			b := uint8(0)
			if (ptr & 0x1FFFFFFF) < uint32(len(bus.RAM)) {
				b = bus.RAM[ptr&0x1FFFFFFF]
			}
			fmt.Printf("final poll state ptr=0x%08X byte0=0x%02X at1c90=0x%08X around=%q\n", ptr, b, binary.BigEndian.Uint32(bus.RAM[0x1C90:]), previewRAMString(bus, (ptr-64)&0x1FFFFFFF, 192))
			dumpRAMRange(bus, 0x000F4800, 0x1200, "debug_parser_stream.bin")
			dumpRAMRange(bus, 0x0024A900, 0x220, "debug_8024a900.bin")
			dumpRAMRange(bus, 0x0024B500, 0x400, "debug_8024b500.bin")
			dumpRAMRange(bus, 0x0024B700, 0x220, "debug_8024b700.bin")
		}
		if cpu.PC >= 0x802C8000 && cpu.PC < 0x802C9000 {
			for r := 0; r < 32; r += 4 {
				fmt.Printf("  final2 r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X r%02d=0x%016X\n",
					r, cpu.Regs[r], r+1, cpu.Regs[r+1], r+2, cpu.Regs[r+2], r+3, cpu.Regs[r+3])
			}
			dumpRAMRange(bus, 0x002C8000, 0x800, "debug_802c8000.bin")
		}
		if cpu.COP0[EPC] >= 0x80225C00 && cpu.COP0[EPC] < 0x80225E00 {
			dumpRAMRange(bus, 0x00225C00, 0x200, "debug_80225c00.bin")
		}
		for i := 0; i < len(recent); i++ {
			entry := recent[(200_000_000+i)%len(recent)]
			if entry.step != 0 {
				fmt.Printf("  recent step=%d PC=0x%08X inst=0x%08X\n", entry.step, entry.pc, entry.inst)
			}
		}
		fmt.Printf("PC==0xBFC00200 hits: %d\n", handlerEntryHits)
		fmt.Printf("PC==0xBFC00380 hits: %d\n", generalHandlerHits)
	}()
	fmt.Printf("Stopped at PC=0x%08X Status=0x%08X\n", cpu.PC, cpu.COP0[Status])
	fmt.Printf("Final r9/t1=0x%016X (%d)\n", cpu.Regs[9], cpu.Regs[9])
	fmt.Printf("VID_NSTART=0x%08X\n", bus.Asic.VIDNStart)
	if bus.IDE != nil {
		fmt.Printf("IDE sector reads: %d\n", bus.IDE.ReadCount)
	}
	fmt.Printf("ASIC final IRQ BusStat=0x%08X BusEn=0x%08X TimStat=0x%08X TimEn=0x%08X TimerCompare=0x%08X SysTimerNext=0x%016X Cycles=0x%016X\n", bus.Asic.BusIntStat, bus.Asic.BusIntEnable, bus.Asic.TimIntStat, bus.Asic.TimIntEnable, bus.Asic.TimerCompare, bus.Asic.SysTimerNext, bus.Asic.Cycles)
	fmt.Printf("Written ASIC registers:\n")
	offs := make([]uint32, 0, len(bus.Asic.WriteLog))
	for o := range bus.Asic.WriteLog {
		offs = append(offs, o)
	}
	sort.Slice(offs, func(i, j int) bool { return offs[i] < offs[j] })
	for _, o := range offs {
		fmt.Printf("  0x%04X = 0x%08X\n", o, bus.Asic.WriteLog[o])
	}
	bus.Asic.PrintTopReads(10)

	read32 := func(off uint32) uint32 {
		if off+3 < RAM_SIZE {
			return binary.BigEndian.Uint32(bus.RAM[off:])
		}
		return 0
	}
	fmt.Printf("Key RAM: [0x400]=0x%08X [0x2EA4]=0x%08X\n", read32(0x400), read32(0x2EA4))
	fmt.Printf("TLB entries used: ")
	used := 0
	for i := 0; i < 48; i++ {
		if cpu.TLB[i].Lo0 != 0 || cpu.TLB[i].Lo1 != 0 {
			used++
		}
	}
	fmt.Printf("%d\n", used)
	bus.PrintTopPollAddrs(12)
}

func loadBootROM(bus *Bus) error {
	data, err := os.ReadFile(bootROMPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", bootROMPath, err)
	}
	if len(data) > 4*1024*1024 {
		data = data[:4*1024*1024]
	}
	copy(bus.ROM[0xC00000:], data)
	bus.ROMSz = uint32(len(data))
	fmt.Printf("Boot ROM: %d bytes\n", len(data))
	return nil
}

func loadFirmwareToFlash(bus *Bus, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	for _, off := range []int64{0x80600} {
		if off+0x44 > fi.Size() {
			continue
		}
		header := make([]byte, 0x44)
		if _, err := f.ReadAt(header, off); err != nil {
			continue
		}
		swap16(header)
		opcode := (binary.BigEndian.Uint32(header[0x00:]) >> 26) & 0x3F
		romLen := (binary.BigEndian.Uint32(header[0x0C:]) - 3) * 4
		buildNum := binary.BigEndian.Uint32(header[0x14:])
		loadAddr := binary.BigEndian.Uint32(header[0x30:])
		if opcode <= 0x04 && buildNum > 0 && buildNum < 100000 && loadAddr >= 0x80000000 && loadAddr < 0x90000000 {
			readSize := romLen
			if off+int64(readSize) > fi.Size() {
				readSize = uint32(fi.Size() - off)
			}
			fw := make([]byte, readSize)
			if _, err := f.ReadAt(fw, off); err != nil {
				continue
			}
			swap16(fw)
			flashOff := uint32(0x1F000000) - ROM_BASE
			copyLen := len(fw)
			if int(flashOff)+copyLen > len(bus.ROM) {
				copyLen = len(bus.ROM) - int(flashOff)
			}
			copy(bus.ROM[flashOff:], fw[:copyLen])
			ramOff := loadAddr & 0x1FFFFFFF
			if ramOff < RAM_SIZE {
				ramCopyLen := len(fw)
				if int(ramOff)+ramCopyLen > len(bus.RAM) {
					ramCopyLen = len(bus.RAM) - int(ramOff)
				}
				copy(bus.RAM[ramOff:], fw[:ramCopyLen])
				fmt.Printf("Firmware in RAM: %d bytes at virt 0x%08X\n", ramCopyLen, loadAddr)
			}
			fmt.Printf("Firmware in flash: %d bytes at phys 0x1F000000\n", copyLen)
			return
		}
	}
}

func loadFromChips(bus *Bus, path0, path1 string) error {
	rom0, err := os.ReadFile(path0)
	if err != nil {
		return fmt.Errorf("read %s: %w", path0, err)
	}
	rom1, err := os.ReadFile(path1)
	if err != nil {
		return fmt.Errorf("read %s: %w", path1, err)
	}
	if len(rom0) != len(rom1) {
		return fmt.Errorf("chip sizes must match: %d != %d", len(rom0), len(rom1))
	}
	bus.LoadBootROM(rom0, rom1)
	fmt.Printf("Chips: %d-byte boot ROM interleaved\n", bus.ROMSz)
	return nil
}

func swap16(data []byte) {
	for i := 0; i+1 < len(data); i += 2 {
		data[i], data[i+1] = data[i+1], data[i]
	}
}

func dumpRAMRange(bus *Bus, off uint32, size uint32, path string) {
	if off+size > uint32(len(bus.RAM)) {
		return
	}
	if err := os.WriteFile(path, bus.RAM[off:off+size], 0644); err != nil {
		fmt.Fprintf(os.Stderr, "dump %s: %v\n", path, err)
		return
	}
	fmt.Printf("dumped RAM 0x%08X..0x%08X to %s\n", off, off+size, path)
}

func printRangeSummary(bus *Bus, off uint32, size uint32, label string) {
	if off+size > uint32(len(bus.RAM)) {
		return
	}
	zero, other, c0ed := 0, 0, 0
	unique := make(map[byte]bool)
	for i := uint32(0); i < size; i++ {
		b := bus.RAM[off+i]
		unique[b] = true
		if b == 0 {
			zero++
		} else {
			other++
		}
	}
	for i := uint32(0); i+3 < size; i += 4 {
		if binary.BigEndian.Uint32(bus.RAM[off+i:]) == 0xC0EDBABE {
			c0ed++
		}
	}
	fmt.Printf("RAM summary %s off=0x%08X size=0x%X zero=%d other=%d c0edWords=%d uniqueBytes=%d first32=% X\n",
		label, off, size, zero, other, c0ed, len(unique), bus.RAM[off:off+32])
}

func previewRAMString(bus *Bus, off uint32, limit int) string {
	if off >= uint32(len(bus.RAM)) {
		return ""
	}
	out := make([]byte, 0, limit)
	for i := 0; i < limit && int(off)+i < len(bus.RAM); i++ {
		b := bus.RAM[int(off)+i]
		if b == 0 {
			break
		}
		out = append(out, b)
	}
	return string(out)
}

func shimLocalKey(bus *Bus, bufOff uint32, suffixOff uint32) {
	key := previewRAMString(bus, bufOff, 120)
	suffix := previewRAMString(bus, suffixOff, 16)
	if suffix != "dat" && suffix != "dir" {
		return
	}
	if len(key) < 2 || key[len(key)-2:] != ". " {
		return
	}
	fixed := key[:len(key)-1] + suffix
	if int(bufOff)+len(fixed)+1 > len(bus.RAM) {
		return
	}
	copy(bus.RAM[bufOff:], []byte(fixed))
	bus.RAM[int(bufOff)+len(fixed)] = 0
}

func shimOptionsKey(bus *Bus, bufOff uint32, key string) {
	if key == "" {
		return
	}
	current := previewRAMString(bus, bufOff, 120)
	if current != "file://rom/images/options .gif" {
		return
	}
	fixed := "file://rom/images/options" + key + ".gif"
	if int(bufOff)+len(fixed)+1 > len(bus.RAM) {
		return
	}
	copy(bus.RAM[bufOff:], []byte(fixed))
	bus.RAM[int(bufOff)+len(fixed)] = 0
}
