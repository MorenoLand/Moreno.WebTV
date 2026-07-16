package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const bootROMPath = "roms/bootrom.brom"

const forceEmptyModemScript = false
const forceSchedulerHeadReady = false
const forceBypassModemReadyWait = false
const forceBootEventGate = false
const forceGateSetterCall = false

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
		printfHits := 0
		nativeLookupHits := 0
		nativeResolveHits := 0
		modemNativeHits := 0
		modemDelayLoopHits := 0
		interruptHelperHits := 0
		idleBlockHits := 0
		schedulerDispatchHits := 0
		timerSchedBodyHits := 0
		lateCurrentDispatchHits := 0
		lateExceptionExitHits := 0
		schedCallbackHits := 0
		timerIrqCallbackHits := 0
		currentTaskHits := 0
		currentTaskTraceHits := 0
		currentTaskWindowUntil := 0
		currentTaskWindowHits := 0
		gateFlagTraceHits := 0
		gateFlagFuncHits := 0
		gateQueueCallerHits := 0
		gateQueueSetHits := 0
		currentWaitHits := 0
		bootGateHits := 0
		bootGateSetterHits := 0
		bootGateSetterUntil := 0
		bootGateCodeDumped := false
		bootListHits := 0
		bootListPathHits := 0
		bootListPCs := make(map[uint32]bool)
		gateDecisionHits := 0
		gateDecisionDumped := false
		gateListCallerHits := 0
		gateListCallerDumped := false
		gateListHelperHits := 0
		gateListHelperDumped := false
		gateListBuildHits := 0
		gateListBuildDumped := false
		gateInsertCallerHits := 0
		gateInsertCallerDumped := false
		gateInsertHits := 0
		key2InsertHits := 0
		key2LookupHits := 0
		reg2004ReadHits := 0
		reg2004ReadUntil := 0
		postGateFaultHits := 0
		postGateFaultDumped := false
		nullObjHelperHits := 0
		nullObjHelperDumped := false
		kseg2AllocHits := 0
		kseg2AllocDumped := false
		modemLoopHits := 0
		modemCoreHits := 0
		modemCodeDumped := false
		eventPushCallerHits := 0
		eventPushFuncHits := 0
		devicePtrFuncHits := 0
		devicePtrCodeDumped := false
		timeFuncHits := 0
		schedulerCodeDumped := false
		irqLoopHits := 0
		irqCodeDumped := false
		newLoopDumped := false
		lastResolverKey := ""
		bus.Tracking = true
		modemScriptEmptied := false
		modemReadyWaitPatched := false
		schedUnblocked := false
		gateSetterPatched := false
		gateQueueWord := binary.BigEndian.Uint32(bus.RAM[0x80A8C:])
		for step := 0; step < 650_000_000; step++ {
			if step > 38_000_000 {
				binary.BigEndian.PutUint32(bus.RAM[0x2730:], uint32(bus.Asic.Cycles/100000))
			}
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
				if step <= 200_000_000 {
					printSchedulerNode(bus, head, "head")
					printSchedulerNode(bus, glob, "glob")
					printSchedulerChain(bus, head, "head-chain")
				}
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
			if forceSchedulerHeadReady && !schedUnblocked && step > 1_100_000_000 {
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
			if forceGateSetterCall && !gateSetterPatched && step > 54_000_000 {
				binary.BigEndian.PutUint32(bus.RAM[0x2C6834:], 0)
				gateSetterPatched = true
				fmt.Printf("gate setter branch patched step=%d PC=0x%08X\n", step, cpu.PC)
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
			if cpu.PC == 0x8024C098 && printfHits < 80 {
				printfHits++
				obj := uint32(cpu.Regs[4]) & 0x1FFFFFFF
				w0, w4, w8, wc := uint32(0), uint32(0), uint32(0), uint32(0)
				if obj+0x0F < uint32(len(bus.RAM)) {
					w0 = binary.BigEndian.Uint32(bus.RAM[obj:])
					w4 = binary.BigEndian.Uint32(bus.RAM[obj+4:])
					w8 = binary.BigEndian.Uint32(bus.RAM[obj+8:])
					wc = binary.BigEndian.Uint32(bus.RAM[obj+12:])
				}
				fmt.Printf("ts printf %02d step=%d fmt=%q a0=0x%016X a1=0x%016X a2=0x%016X ra=0x%016X\n",
					printfHits, step, previewRAMString(bus, uint32(cpu.Regs[4])&0x1FFFFFFF, 160), cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[31])
				fmt.Printf("  printf obj words=%08X %08X %08X %08X s0=%q s4=%q s8=%q sc=%q\n",
					w0, w4, w8, wc, previewRAMString(bus, w0&0x1FFFFFFF, 160), previewRAMString(bus, w4&0x1FFFFFFF, 160), previewRAMString(bus, w8&0x1FFFFFFF, 160), previewRAMString(bus, wc&0x1FFFFFFF, 160))
			}
			if cpu.PC == 0x802281DC && nativeLookupHits < 120 {
				key := previewRAMString(bus, uint32(cpu.Regs[4])&0x1FFFFFFF, 96)
				cand := previewRAMString(bus, uint32(cpu.Regs[5])&0x1FFFFFFF, 96)
				if strings.Contains(key, "modem") || strings.Contains(cand, "modem") || strings.Contains(key, "printf") || strings.Contains(cand, "printf") {
					nativeLookupHits++
					fmt.Printf("native lookup %03d step=%d key=%q cand=%q a0=0x%016X a1=0x%016X ra=0x%016X s0=0x%016X s2=0x%016X\n",
						nativeLookupHits, step, key, cand, cpu.Regs[4], cpu.Regs[5], cpu.Regs[31], cpu.Regs[16], cpu.Regs[18])
				}
			}
			if cpu.PC == 0x8024B8D0 && nativeResolveHits < 80 {
				key := previewRAMString(bus, uint32(cpu.Regs[18])&0x1FFFFFFF, 96)
				if strings.Contains(key, "modem") || strings.Contains(key, "printf") {
					nativeResolveHits++
					entry := uint32(cpu.Regs[16]) & 0x1FFFFFFF
					w0, w1, w2 := uint32(0), uint32(0), uint32(0)
					if entry+11 < uint32(len(bus.RAM)) {
						w0 = binary.BigEndian.Uint32(bus.RAM[entry:])
						w1 = binary.BigEndian.Uint32(bus.RAM[entry+4:])
						w2 = binary.BigEndian.Uint32(bus.RAM[entry+8:])
					}
					fmt.Printf("native resolve %02d step=%d key=%q entry=0x%08X w0=0x%08X w1=0x%08X w2=0x%08X v0=0x%016X ra=0x%016X\n",
						nativeResolveHits, step, key, entry, w0, w1, w2, cpu.Regs[2], cpu.Regs[31])
				}
			}
			logModemNative := cpu.PC == 0x8024CA18 || cpu.PC == 0x8024CA4C || cpu.PC == 0x8024CA54 || cpu.PC == 0x8024CA58 || cpu.PC == 0x8024CD48 || cpu.PC == 0x8024CD5C || cpu.PC == 0x8024CD74 || cpu.PC == 0x8024CDAC || cpu.PC == 0x8024CCE8
			if cpu.PC == 0x8024CD20 {
				modemDelayLoopHits++
				logModemNative = modemDelayLoopHits <= 8 || modemDelayLoopHits%500 == 0
			}
			if logModemNative && modemNativeHits < 320 {
				modemNativeHits++
				obj := bus.Read32(0x800013D0)
				ctxObj := uint32(0)
				if (uint32(cpu.Regs[21])&0x1FFFFFFF)+3 < uint32(len(bus.RAM)) {
					ctxObj = binary.BigEndian.Uint32(bus.RAM[uint32(cpu.Regs[21])&0x1FFFFFFF:])
				}
				op := uint32(0)
				cb := uint32(0)
				if (ctxObj&0x1FFFFFFF)+0x87 < uint32(len(bus.RAM)) {
					op = binary.BigEndian.Uint32(bus.RAM[(ctxObj&0x1FFFFFFF)+0x10:])
					cb = binary.BigEndian.Uint32(bus.RAM[(ctxObj&0x1FFFFFFF)+0x84:])
				}
				fmt.Printf("modem native %03d step=%d PC=0x%08X glob=0x%08X ctxObj=0x%08X op=0x%08X cb84=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X s5=0x%016X ra=0x%016X\n",
					modemNativeHits, step, cpu.PC, obj, ctxObj, op, cb, cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[21], cpu.Regs[31])
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
			if cpu.PC == 0x802C6680 && currentTaskHits < 40 {
				currentTaskHits++
				glob := binary.BigEndian.Uint32(bus.RAM[0x1900:])
				fmt.Printf("current task %02d step=%d PC=0x%08X glob=0x%08X flag1909=0x%02X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					currentTaskHits, step, cpu.PC, glob, bus.RAM[0x1909], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
			}
			if step > 54_700_000 && cpu.PC == 0x802C6820 && currentTaskWindowUntil == 0 {
				currentTaskWindowUntil = step + 80
			}
			if step < currentTaskWindowUntil && currentTaskWindowHits < 120 {
				currentTaskWindowHits++
				s0p := uint32(cpu.Regs[16]) & 0x1FFFFFFF
				w0, w4, wc, w3c4 := uint32(0), uint32(0), uint32(0), uint32(0)
				if s0p+0x3C7 < uint32(len(bus.RAM)) {
					w0 = binary.BigEndian.Uint32(bus.RAM[s0p:])
					w4 = binary.BigEndian.Uint32(bus.RAM[s0p+4:])
					wc = binary.BigEndian.Uint32(bus.RAM[s0p+12:])
					w3c4 = binary.BigEndian.Uint32(bus.RAM[s0p+0x3C4:])
				}
				fmt.Printf("current window %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X s0w=%08X/%08X/%08X/%08X list=%08X/%08X/%08X/%08X\n",
					currentTaskWindowHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31], w0, w4, wc, w3c4, binary.BigEndian.Uint32(bus.RAM[0x7E930:]), binary.BigEndian.Uint32(bus.RAM[0x7E938:]), binary.BigEndian.Uint32(bus.RAM[0x7E93C:]), binary.BigEndian.Uint32(bus.RAM[0x7E948:]))
			}
			if (cpu.PC == 0x802C66B4 || cpu.PC == 0x802C66BC || cpu.PC == 0x802C66D0 || cpu.PC == 0x802C66E8 || cpu.PC == 0x802C6700 || cpu.PC == 0x802C6724 || cpu.PC == 0x802C672C || cpu.PC == 0x802C6734 || cpu.PC == 0x802C6748 || cpu.PC == 0x802C6750 || cpu.PC == 0x802C6768 || cpu.PC == 0x802C6770 || cpu.PC == 0x802C6798 || cpu.PC == 0x802C67B0 || cpu.PC == 0x802C67E0 || cpu.PC == 0x802C67E8 || cpu.PC == 0x802C67EC || cpu.PC == 0x802C67F4 || cpu.PC == 0x802C6800 || cpu.PC == 0x802C6808 || cpu.PC == 0x802C6820 || cpu.PC == 0x802C6828 || cpu.PC == 0x802C6830 || cpu.PC == 0x802C6834 || cpu.PC == 0x802C6838 || cpu.PC == 0x802C683C || cpu.PC == 0x802C6848 || cpu.PC == 0x802C6858 || cpu.PC == 0x802C6C84 || cpu.PC == 0x8022A198 || cpu.PC == 0x8022A19C || cpu.PC == 0x8022A1A8) && currentTaskTraceHits < 180 {
				currentTaskTraceHits++
				fmt.Printf("current path %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					currentTaskTraceHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
			}
			if (cpu.PC == 0x802C67E8 || cpu.PC == 0x802C67EC || cpu.PC == 0x802C67F4 || cpu.PC == 0x802C6800 || cpu.PC == 0x802C6808 || cpu.PC == 0x802C6828 || cpu.PC == 0x802C6830 || cpu.PC == 0x802C6834 || cpu.PC == 0x802C6838) && gateFlagTraceHits < 80 {
				gateFlagTraceHits++
				s0p := uint32(cpu.Regs[16]) & 0x1FFFFFFF
				s1p := uint32(cpu.Regs[17]) & 0x1FFFFFFF
				s0c4, s0c8, s0cc, s1c4 := uint32(0), uint32(0), uint32(0), uint32(0)
				if s0p+0x3CF < uint32(len(bus.RAM)) {
					s0c4 = binary.BigEndian.Uint32(bus.RAM[s0p+0x3C4:])
					s0c8 = binary.BigEndian.Uint32(bus.RAM[s0p+0x3C8:])
					s0cc = binary.BigEndian.Uint32(bus.RAM[s0p+0x3CC:])
				}
				if s1p+0x3C7 < uint32(len(bus.RAM)) {
					s1c4 = binary.BigEndian.Uint32(bus.RAM[s1p+0x3C4:])
				}
				fmt.Printf("gate flags %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X s0+3C4=%08X s0+3C8=%08X s0+3CC=%08X s1+3C4=%08X g17A0=%08X\n",
					gateFlagTraceHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], s0c4, s0c8, s0cc, s1c4, binary.BigEndian.Uint32(bus.RAM[0x17A0:]))
			}
			if cpu.PC >= 0x802C5244 && cpu.PC < 0x802C5320 && gateFlagFuncHits < 80 {
				gateFlagFuncHits++
				fmt.Printf("gate flag func %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X g2170=%08X g2174=%08X g2178=%08X g217C=%08X ra=0x%016X\n",
					gateFlagFuncHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], binary.BigEndian.Uint32(bus.RAM[0x2170:]), binary.BigEndian.Uint32(bus.RAM[0x2174:]), binary.BigEndian.Uint32(bus.RAM[0x2178:]), binary.BigEndian.Uint32(bus.RAM[0x217C:]), cpu.Regs[31])
			}
			if cpu.PC >= 0x802C52A4 && cpu.PC < 0x802C52E8 && gateQueueSetHits < 160 {
				gateQueueSetHits++
				fmt.Printf("gate queue set %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X q88=%08X q8C=%08X q90=%08X q94=%08X q98=%08X g2178=%08X ra=0x%016X\n",
					gateQueueSetHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], binary.BigEndian.Uint32(bus.RAM[0x80A88:]), binary.BigEndian.Uint32(bus.RAM[0x80A8C:]), binary.BigEndian.Uint32(bus.RAM[0x80A90:]), binary.BigEndian.Uint32(bus.RAM[0x80A94:]), binary.BigEndian.Uint32(bus.RAM[0x80A98:]), binary.BigEndian.Uint32(bus.RAM[0x2178:]), cpu.Regs[31])
			}
			if (cpu.PC == 0x80234F50 || cpu.PC == 0x802B4490 || cpu.PC == 0x802C6800) && gateQueueCallerHits < 40 {
				gateQueueCallerHits++
				fmt.Printf("gate queue caller %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X q80A8C=%08X\n",
					gateQueueCallerHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x80A8C:]))
			}
			if (cpu.PC == 0x802C6798 || cpu.PC == 0x802C6808 || cpu.PC == 0x802C683C || cpu.PC == 0x802C6C84) && currentTaskTraceHits < 200 {
				currentTaskTraceHits++
				fmt.Printf("gate setter call %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					currentTaskTraceHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
			}
			if cpu.PC >= 0x802C5F80 && cpu.PC < 0x802C6040 && modemLoopHits < 160 {
				modemLoopHits++
				fmt.Printf("modem loop %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X rx=%d stage=%d\n",
					modemLoopHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31], len(bus.ModemRX), bus.ModemStage)
			}
			if (cpu.PC == 0x8020DCA4 || cpu.PC == 0x8020DF18 || cpu.PC == 0x8020E0A4 || cpu.PC == 0x80233750 || cpu.PC == 0x802C5F00 || cpu.PC == 0x802C5F2C) && eventPushCallerHits < 80 {
				eventPushCallerHits++
				fmt.Printf("event push caller %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X g1E74=%08X\n",
					eventPushCallerHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x1E74:]))
			}
			if cpu.PC >= 0x802C5E84 && cpu.PC < 0x802C5F50 && eventPushFuncHits < 160 {
				eventPushFuncHits++
				fmt.Printf("event push func hit %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X g1E74=%08X\n",
					eventPushFuncHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x1E74:]))
			}
			if cpu.PC >= 0x80222E58 && cpu.PC < 0x80222F20 && devicePtrFuncHits < 160 {
				devicePtrFuncHits++
				fmt.Printf("device ptr func hit %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X low400=%08X low588=%08X\n",
					devicePtrFuncHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x400:]), binary.BigEndian.Uint32(bus.RAM[0x588:]))
				if !devicePtrCodeDumped {
					devicePtrCodeDumped = true
					printCodeRange(bus, 0x80222E58, 0x80222F20, "device ptr func")
				}
			}
			if cpu.PC >= 0x80211260 && cpu.PC < 0x802112D0 && !newLoopDumped {
				newLoopDumped = true
				fmt.Printf("new loop step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X s3=0x%016X ra=0x%016X\n",
					step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19], cpu.Regs[31])
				printCodeRange(bus, 0x80211260, 0x802113A0, "new loop")
			}
			if ((cpu.PC >= 0x8020C380 && cpu.PC < 0x8020C400) || (cpu.PC >= 0x8024CB80 && cpu.PC < 0x8024CC00)) && (modemCoreHits < 60 || bus.ModemStage >= 3) && modemCoreHits < 260 {
				modemCoreHits++
				fmt.Printf("modem core %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X s3=0x%016X ra=0x%016X rx=%d stage=%d out=%q\n",
					modemCoreHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19], cpu.Regs[31], len(bus.ModemRX), bus.ModemStage, previewPhysString(bus, uint32(cpu.Regs[17])&0x1FFFFFFF, 96))
				if !modemCodeDumped {
					modemCodeDumped = true
					printCodeRange(bus, 0x8020C360, 0x8020C400, "modem core")
					printCodeRange(bus, 0x8024CB80, 0x8024CC10, "modem core")
				}
			}
			if cpu.PC >= 0x8030AD78 && cpu.PC < 0x8030ADE0 && timeFuncHits < 120 {
				timeFuncHits++
				fmt.Printf("time func %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X ra=0x%016X asic48=0x%08X c1cf0=0x%08X c1cf4=0x%08X\n",
					timeFuncHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[31], bus.Asic.Read(0x48), binary.BigEndian.Uint32(bus.RAM[0x1CF0:]), binary.BigEndian.Uint32(bus.RAM[0x1CF4:]))
			}
			if (cpu.PC == 0x802C84DC || cpu.PC == 0x802C84E8 || cpu.PC == 0x802C84F4 || cpu.PC == 0x802C8500 || cpu.PC == 0x802C8510 || cpu.PC == 0x802C8518) && currentWaitHits < 120 {
				currentWaitHits++
				fmt.Printf("current wait %02d step=%d PC=0x%08X inst=0x%08X b224E=0x%02X b2240=0x%02X v0=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X ra=0x%016X\n",
					currentWaitHits, step, cpu.PC, bus.Read32(cpu.PC), bus.RAM[0x224E], bus.RAM[0x2240], cpu.Regs[2], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[31])
				if currentWaitHits == 1 {
					printCodeRange(bus, 0x802C84C0, 0x802C8540, "current wait")
				}
				if forceBootEventGate && currentWaitHits == 12 {
					bus.RAM[0x224E] = 1
					fmt.Printf("forced b224E=1 at step=%d\n", step)
				}
			}
			if (cpu.PC == 0x802C73C0 || cpu.PC == 0x802C73C8 || cpu.PC == 0x802C73D0 || cpu.PC == 0x802C7440 || cpu.PC == 0x802C7450 || cpu.PC == 0x802C74B8) && bootGateHits < 120 {
				bootGateHits++
				fmt.Printf("boot gate %02d step=%d PC=0x%08X inst=0x%08X b224E=0x%02X b2240=0x%02X v0=0x%016X a0=0x%016X a1=0x%016X ra=0x%016X\n",
					bootGateHits, step, cpu.PC, bus.Read32(cpu.PC), bus.RAM[0x224E], bus.RAM[0x2240], cpu.Regs[2], cpu.Regs[4], cpu.Regs[5], cpu.Regs[31])
				if !bootGateCodeDumped {
					bootGateCodeDumped = true
					printCodeRange(bus, 0x802C7380, 0x802C7460, "boot gate")
					printCodeRange(bus, 0x802B3F80, 0x802B42A0, "boot gate callee")
				}
			}
			if cpu.PC == 0x802C7464 && bootGateSetterHits == 0 {
				bootGateSetterUntil = step + 2000
			}
			if step < bootGateSetterUntil && bootGateSetterHits < 300 {
				bootGateSetterHits++
				fmt.Printf("boot gate setter %02d step=%d PC=0x%08X inst=0x%08X b224E=0x%02X b2240=0x%02X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X ra=0x%016X\n",
					bootGateSetterHits, step, cpu.PC, bus.Read32(cpu.PC), bus.RAM[0x224E], bus.RAM[0x2240], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[31])
				if bootGateSetterHits == 1 {
					printCodeRange(bus, 0x802C7460, 0x802C74D0, "boot gate setter")
				}
			}
			if cpu.PC >= 0x802B40F0 && cpu.PC < 0x802B4140 && bootListPathHits < 200 {
				bootListPathHits++
				fmt.Printf("boot list %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X s3=0x%016X ra=0x%016X g1E70=%08X g1E74=%08X acb8=%08X\n",
					bootListPathHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x1E70:]), binary.BigEndian.Uint32(bus.RAM[0x1E74:]), binary.BigEndian.Uint32(bus.RAM[0xACB8:]))
			}
			if cpu.PC >= 0x802B4000 && cpu.PC < 0x802B4800 && !bootListPCs[cpu.PC] && len(bootListPCs) < 240 {
				bootListPCs[cpu.PC] = true
				fmt.Printf("list pc step=%d PC=0x%08X inst=0x%08X v0=0x%016X a0=0x%016X a1=0x%016X ra=0x%016X g1e74=0x%08X\n",
					step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[4], cpu.Regs[5], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x1E74:]))
			}
			if step > 54_000_000 && cpu.PC >= 0x80229BE8 && cpu.PC < 0x80229C80 && gateDecisionHits < 120 {
				gateDecisionHits++
				fmt.Printf("gate decision %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X ra=0x%016X\n",
					gateDecisionHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[31])
				if !gateDecisionDumped {
					gateDecisionDumped = true
					printCodeRange(bus, 0x80229BE8, 0x80229C80, "gate decision")
				}
			}
			if cpu.PC >= 0x802C3080 && cpu.PC < 0x802C3280 && gateListCallerHits < 260 {
				gateListCallerHits++
				fmt.Printf("gate list caller %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X s3=0x%016X s4=0x%016X s5=0x%016X ra=0x%016X\n",
					gateListCallerHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19], cpu.Regs[20], cpu.Regs[21], cpu.Regs[31])
				if !gateListCallerDumped {
					gateListCallerDumped = true
					printCodeRange(bus, 0x802C3080, 0x802C3280, "gate list caller")
				}
			}
			if cpu.PC >= 0x80229D40 && cpu.PC < 0x80229E10 && gateListHelperHits < 160 {
				gateListHelperHits++
				fmt.Printf("gate list helper %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X ram2004=0x%08X\n",
					gateListHelperHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x2004:]))
				if !gateListHelperDumped {
					gateListHelperDumped = true
					printCodeRange(bus, 0x80229D40, 0x80229E10, "gate list helper")
				}
			}
			if cpu.PC == 0x80229BE8 && cpu.Regs[5] == 2 && key2LookupHits < 80 {
				key2LookupHits++
				a0p := uint32(cpu.Regs[4]) & 0x1FFFFFFF
				w0, w4, w8 := uint32(0), uint32(0), uint32(0)
				if a0p+11 < uint32(len(bus.RAM)) {
					w0 = binary.BigEndian.Uint32(bus.RAM[a0p:])
					w4 = binary.BigEndian.Uint32(bus.RAM[a0p+4:])
					w8 = binary.BigEndian.Uint32(bus.RAM[a0p+8:])
				}
				fmt.Printf("key2 lookup %02d step=%d a0=0x%016X head=%08X/%08X/%08X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					key2LookupHits, step, cpu.Regs[4], w0, w4, w8, cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
				if key2LookupHits == 1 {
					printListChain(bus, uint32(cpu.Regs[4]), "key2 list", 8)
				}
			}
			if cpu.PC == 0x80229E10 && (uint32(cpu.Regs[4])&0x1FFFFFFF) == 0x2004 {
				reg2004ReadUntil = step + 80
			}
			if step < reg2004ReadUntil && reg2004ReadHits < 160 {
				if cpu.PC >= 0x80229E10 && cpu.PC < 0x80229E80 {
					reg2004ReadHits++
					fmt.Printf("reg2004 helper %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X ra=0x%016X ram2004=0x%08X\n",
						reg2004ReadHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x2004:]))
				}
			}
			if cpu.PC >= 0x8026A060 && cpu.PC < 0x8026A0E0 && postGateFaultHits < 120 {
				postGateFaultHits++
				fmt.Printf("post gate fault %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X\n",
					postGateFaultHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31])
				if !postGateFaultDumped {
					postGateFaultDumped = true
					printCodeRange(bus, 0x8026A060, 0x8026A0E0, "post gate fault")
				}
			}
			if cpu.PC >= 0x80268E20 && cpu.PC < 0x80269130 && nullObjHelperHits < 240 {
				nullObjHelperHits++
				fmt.Printf("null obj helper %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X\n",
					nullObjHelperHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31])
				if !nullObjHelperDumped {
					nullObjHelperDumped = true
					printCodeRange(bus, 0x80268E20, 0x80269130, "null obj helper")
				}
			}
			if cpu.PC >= 0x802C3280 && cpu.PC < 0x802C3310 && gateListBuildHits < 160 {
				gateListBuildHits++
				fmt.Printf("gate list build %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					gateListBuildHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
				if !gateListBuildDumped {
					gateListBuildDumped = true
					printCodeRange(bus, 0x802C3280, 0x802C3310, "gate list build")
				}
			}
			if step > 38_500_000 && cpu.PC >= 0x80229A60 && cpu.PC < 0x80229B70 && gateInsertCallerHits < 220 {
				gateInsertCallerHits++
				fmt.Printf("gate insert caller %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X s3=0x%016X s4=0x%016X ra=0x%016X\n",
					gateInsertCallerHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19], cpu.Regs[20], cpu.Regs[31])
				if !gateInsertCallerDumped {
					gateInsertCallerDumped = true
					printCodeRange(bus, 0x80229A60, 0x80229B70, "gate insert caller")
				}
			}
			if cpu.PC >= 0x80229B70 && cpu.PC < 0x80229BE0 && gateInsertHits < 120 {
				a0p := uint32(cpu.Regs[4]) & 0x1FFFFFFF
				if a0p >= 0x7E900 && a0p < 0x7EA00 {
					gateInsertHits++
					fmt.Printf("gate insert %03d step=%d PC=0x%08X inst=0x%08X a0=0x%016X a1=0x%016X a2=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X\n",
						gateInsertHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31])
				}
			}
			if cpu.PC >= 0x80229B70 && cpu.PC < 0x80229BE0 && cpu.Regs[17] == 2 && key2InsertHits < 80 {
				key2InsertHits++
				fmt.Printf("key2 insert %02d step=%d PC=0x%08X inst=0x%08X a0=0x%016X a1=0x%016X a2=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X\n",
					key2InsertHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31])
			}
			if cpu.PC >= 0x8024E760 && cpu.PC <= 0x8024E7A4 && bootListHits < 220 {
				bootListHits++
				fmt.Printf("global clear %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X ra=0x%016X g1e74=0x%08X\n",
					bootListHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x1E74:]))
			}
			if cpu.PC >= 0x8022A430 && cpu.PC < 0x8022A500 && lateLoopReports < 40 {
				lateLoopReports++
				fmt.Printf("late loop %d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X t0=0x%016X t1=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					lateLoopReports, step, cpu.PC, bus.Read32(cpu.PC),
					cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[8], cpu.Regs[9], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
			}
			if cpu.PC >= 0x8022A430 && cpu.PC < 0x8022A500 && !schedulerCodeDumped {
				schedulerCodeDumped = true
				printCodeRange(bus, 0x8022A240, 0x8022A380, "sched code")
				printCodeRange(bus, 0x8022A430, 0x8022A4D0, "sched code")
				printCodeRange(bus, 0x8022A7A0, 0x8022A820, "sched code")
				printCodeRange(bus, 0x8022ADE0, 0x8022AE40, "sched code")
			}
			if cpu.PC >= 0x80221500 && cpu.PC < 0x80221620 && !irqCodeDumped {
				irqCodeDumped = true
				printCodeRange(bus, 0x802214E0, 0x80221640, "irq code")
			}
			if cpu.PC >= 0x80221500 && cpu.PC < 0x80221620 && irqLoopHits < 120 {
				irqLoopHits++
				fmt.Printf("irq loop %03d step=%d PC=0x%08X inst=0x%08X Status=0x%08X Cause=0x%08X Count=0x%08X Compare=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X t9=0x%016X k0=0x%016X k1=0x%016X ra=0x%016X busStat=0x%08X busEn=0x%08X timStat=0x%08X timEn=0x%08X rioStat=0x%08X rioEn=0x%08X\n",
					irqLoopHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[Status], cpu.COP0[Cause], cpu.COP0[Count], cpu.COP0[Compare], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[25], cpu.Regs[26], cpu.Regs[27], cpu.Regs[31], bus.Asic.BusIntStat, bus.Asic.BusIntEnable, bus.Asic.TimIntStat, bus.Asic.TimIntEnable, bus.Asic.RIOIntStat, bus.Asic.RIOIntEnable)
			}
			if cpu.PC >= 0x802220B8 && cpu.PC <= 0x802220C8 && interruptHelperHits < 80 {
				interruptHelperHits++
				fmt.Printf("int helper %02d step=%d PC=0x%08X inst=0x%08X Status=0x%08X Cause=0x%08X Count=0x%08X Compare=0x%08X a0=0x%016X ra=0x%016X\n",
					interruptHelperHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[Status], cpu.COP0[Cause], cpu.COP0[Count], cpu.COP0[Compare], cpu.Regs[4], cpu.Regs[31])
			}
			if cpu.PC >= 0x80221A60 && cpu.PC <= 0x80221AB0 && idleBlockHits < 80 {
				idleBlockHits++
				fmt.Printf("idle block %02d step=%d PC=0x%08X inst=0x%08X Status=0x%08X Cause=0x%08X Count=0x%08X Compare=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X ra=0x%016X busStat=0x%08X busEn=0x%08X timStat=0x%08X timEn=0x%08X\n",
					idleBlockHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[Status], cpu.COP0[Cause], cpu.COP0[Count], cpu.COP0[Compare], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[31], bus.Asic.BusIntStat, bus.Asic.BusIntEnable, bus.Asic.TimIntStat, bus.Asic.TimIntEnable)
			}
			if step > 500_000_000 && cpu.PC >= 0x8022A180 && cpu.PC < 0x8022A220 && lateCurrentDispatchHits < 120 {
				lateCurrentDispatchHits++
				head := binary.BigEndian.Uint32(bus.RAM[0x18F0:])
				glob := binary.BigEndian.Uint32(bus.RAM[0x1900:])
				h0, h4, h8, hc, h10, h20 := uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0)
				if head >= 0x80000000 && head < 0x80800000 {
					off := head - 0x80000000
					if off+0x24 <= uint32(len(bus.RAM)) {
						h0 = binary.BigEndian.Uint32(bus.RAM[off:])
						h4 = binary.BigEndian.Uint32(bus.RAM[off+4:])
						h8 = binary.BigEndian.Uint32(bus.RAM[off+8:])
						hc = binary.BigEndian.Uint32(bus.RAM[off+0xC:])
						h10 = binary.BigEndian.Uint32(bus.RAM[off+0x10:])
						h20 = binary.BigEndian.Uint32(bus.RAM[off+0x20:])
					}
				}
				fmt.Printf("late current dispatch %03d step=%d PC=0x%08X inst=0x%08X Status=0x%08X Cause=0x%08X Count=0x%08X Compare=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X head=0x%08X glob=0x%08X h0=0x%08X h4=0x%08X h8=0x%08X hc=0x%08X h10=0x%08X h20=0x%08X r1909=0x%02X\n",
					lateCurrentDispatchHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[Status], cpu.COP0[Cause], cpu.COP0[Count], cpu.COP0[Compare], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31], head, glob, h0, h4, h8, hc, h10, h20, bus.RAM[0x1909])
				if lateCurrentDispatchHits == 1 {
					printCodeRange(bus, 0x8022A180, 0x8022A220, "late current dispatch")
				}
			}
			if step > 500_000_000 && cpu.PC >= 0x80221A70 && cpu.PC < 0x80221AC0 && lateExceptionExitHits < 120 {
				lateExceptionExitHits++
				fmt.Printf("late exception exit %03d step=%d PC=0x%08X inst=0x%08X EPC=0x%08X epcInst=0x%08X Status=0x%08X Cause=0x%08X Count=0x%08X Compare=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X sp=0x%016X ra=0x%016X\n",
					lateExceptionExitHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[EPC], bus.Read32(cpu.COP0[EPC]), cpu.COP0[Status], cpu.COP0[Cause], cpu.COP0[Count], cpu.COP0[Compare], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[29], cpu.Regs[31])
				if lateExceptionExitHits == 1 {
					printCodeRange(bus, 0x80221A70, 0x80221AC0, "late exception exit")
				}
			}
			if cpu.PC >= 0x8022A680 && cpu.PC <= 0x8022A7C0 && schedulerDispatchHits < 120 {
				schedulerDispatchHits++
				fmt.Printf("sched dispatch %03d step=%d PC=0x%08X inst=0x%08X Status=0x%08X Cause=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X\n",
					schedulerDispatchHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[Status], cpu.COP0[Cause], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31])
				if schedulerDispatchHits == 1 {
					printCodeRange(bus, 0x8022A600, 0x8022A720, "timer sched")
					dumpRAMRange(bus, 0x0022A600, 0x200, "debug_8022a600.bin")
				}
			}
			if cpu.PC >= 0x8022A600 && cpu.PC < 0x8022A6A0 && timerSchedBodyHits < 140 && (timerSchedBodyHits < 40 || step > 500_000_000) {
				timerSchedBodyHits++
				head := binary.BigEndian.Uint32(bus.RAM[0x18F0:])
				glob := binary.BigEndian.Uint32(bus.RAM[0x1900:])
				fmt.Printf("timer sched body %03d step=%d PC=0x%08X inst=0x%08X Count=0x%08X Compare=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X ra=0x%016X head=0x%08X glob=0x%08X\n",
					timerSchedBodyHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[Count], cpu.COP0[Compare], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[31], head, glob)
			}
			if (cpu.PC == 0x8022A7A4 || cpu.PC == 0x802C6680 || cpu.PC == 0x80234F20 || cpu.PC == 0x802B4480) && schedCallbackHits < 80 {
				schedCallbackHits++
				fmt.Printf("sched callback %02d step=%d PC=0x%08X inst=0x%08X v0=0x%016X a0=0x%016X a1=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X\n",
					schedCallbackHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[4], cpu.Regs[5], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31])
			}
			if cpu.PC >= 0x802112F0 && cpu.PC < 0x80211330 && timerIrqCallbackHits < 120 {
				timerIrqCallbackHits++
				fmt.Printf("timer irq cb %03d step=%d PC=0x%08X inst=0x%08X Count=0x%08X Compare=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X ra=0x%016X head=0x%08X glob=0x%08X\n",
					timerIrqCallbackHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.COP0[Count], cpu.COP0[Compare], cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x18F0:]), binary.BigEndian.Uint32(bus.RAM[0x1900:]))
				if timerIrqCallbackHits == 1 {
					printCodeRange(bus, 0x802112E0, 0x80211380, "timer irq cb")
				}
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
			if cpu.PC >= 0x80213F80 && cpu.PC < 0x80214020 && kseg2AllocHits < 120 {
				kseg2AllocHits++
				fmt.Printf("kseg2 alloc %03d step=%d PC=0x%08X inst=0x%08X v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X ra=0x%016X g16A0=0x%08X g17A0=0x%08X\n",
					kseg2AllocHits, step, cpu.PC, bus.Read32(cpu.PC), cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[31], binary.BigEndian.Uint32(bus.RAM[0x16A0:]), binary.BigEndian.Uint32(bus.RAM[0x17A0:]))
				if !kseg2AllocDumped {
					kseg2AllocDumped = true
					printCodeRange(bus, 0x80213F80, 0x80214020, "kseg2 alloc")
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
			newGateQueueWord := binary.BigEndian.Uint32(bus.RAM[0x80A8C:])
			if newGateQueueWord != gateQueueWord {
				fmt.Printf("gate queue word changed step=%d PC=0x%08X inst=0x%08X old=%08X new=%08X ra=0x%016X v0=0x%016X a0=0x%016X a1=0x%016X\n",
					step, beforePC, beforeInst, gateQueueWord, newGateQueueWord, cpu.Regs[31], cpu.Regs[2], cpu.Regs[4], cpu.Regs[5])
				gateQueueWord = newGateQueueWord
			}
			if cpu.Regs[20] != beforeS4 && beforePC >= 0x80200000 && s4Changes < 16 {
				s4Changes++
				fmt.Printf("s4 change %d step=%d PC=0x%08X inst=0x%08X old=0x%016X new=0x%016X\n",
					s4Changes, step, beforePC, beforeInst, beforeS4, cpu.Regs[20])
			}
		}
		fmt.Printf("Final instruction: PC=0x%08X inst=0x%08X EPC=0x%08X epcInst=0x%08X BadVAddr=0x%08X Cause=0x%08X Status=0x%08X Count=0x%08X Compare=0x%08X\n",
			cpu.PC, bus.Read32(cpu.PC), cpu.COP0[EPC], bus.Read32(cpu.COP0[EPC]), cpu.COP0[BadVAddr], cpu.COP0[Cause], cpu.COP0[Status], cpu.COP0[Count], cpu.COP0[Compare])
		fmt.Printf("Final regs v0=0x%016X v1=0x%016X a0=0x%016X a1=0x%016X a2=0x%016X a3=0x%016X s0=0x%016X s1=0x%016X s2=0x%016X s3=0x%016X ra=0x%016X\n",
			cpu.Regs[2], cpu.Regs[3], cpu.Regs[4], cpu.Regs[5], cpu.Regs[6], cpu.Regs[7], cpu.Regs[16], cpu.Regs[17], cpu.Regs[18], cpu.Regs[19], cpu.Regs[31])
		fmt.Printf("Final gate 2240=%02X 2241=%02X 2242=%02X 2243=%02X 2248=%02X 2249=%02X 224A=%02X 224D=%02X 224E=%02X\n",
			bus.RAM[0x2240], bus.RAM[0x2241], bus.RAM[0x2242], bus.RAM[0x2243], bus.RAM[0x2248], bus.RAM[0x2249], bus.RAM[0x224A], bus.RAM[0x224D], bus.RAM[0x224E])
		fmt.Printf("Final gate queue 80A88=%08X 80A8C=%08X 80A90=%08X 80A94=%08X 80A98=%08X\n",
			binary.BigEndian.Uint32(bus.RAM[0x80A88:]), binary.BigEndian.Uint32(bus.RAM[0x80A8C:]), binary.BigEndian.Uint32(bus.RAM[0x80A90:]), binary.BigEndian.Uint32(bus.RAM[0x80A94:]), binary.BigEndian.Uint32(bus.RAM[0x80A98:]))
		fmt.Printf("Final eventq 1E70=%08X 1E74=%08X ACB8=%08X ACC0=%08X ACC4=%08X\n",
			binary.BigEndian.Uint32(bus.RAM[0x1E70:]), binary.BigEndian.Uint32(bus.RAM[0x1E74:]), binary.BigEndian.Uint32(bus.RAM[0xACB8:]), binary.BigEndian.Uint32(bus.RAM[0xACC0:]), binary.BigEndian.Uint32(bus.RAM[0xACC4:]))
		printListChain(bus, 0x8007E930, "Final gate list", 8)
		eventTable := binary.BigEndian.Uint32(bus.RAM[0x25D4:]) & 0x1FFFFFFF
		if eventTable+0x60 < uint32(len(bus.RAM)) {
			fmt.Printf("Final event table ptr=0x%08X e0=%08X/%08X e1=%08X/%08X e2=%08X/%08X e3=%08X/%08X\n",
				eventTable,
				binary.BigEndian.Uint32(bus.RAM[eventTable:]), binary.BigEndian.Uint32(bus.RAM[eventTable+0x14:]),
				binary.BigEndian.Uint32(bus.RAM[eventTable+0x18:]), binary.BigEndian.Uint32(bus.RAM[eventTable+0x2C:]),
				binary.BigEndian.Uint32(bus.RAM[eventTable+0x30:]), binary.BigEndian.Uint32(bus.RAM[eventTable+0x44:]),
				binary.BigEndian.Uint32(bus.RAM[eventTable+0x48:]), binary.BigEndian.Uint32(bus.RAM[eventTable+0x5C:]))
			printEventCallbacks(bus, eventTable, 0x300)
		}
		fmt.Printf("Final dispatcher masks=%08X,%08X,%08X,%08X,%08X cbs=%08X,%08X,%08X,%08X,%08X\n",
			binary.BigEndian.Uint32(bus.RAM[0x1430:]), binary.BigEndian.Uint32(bus.RAM[0x1438:]), binary.BigEndian.Uint32(bus.RAM[0x1440:]), binary.BigEndian.Uint32(bus.RAM[0x1448:]), binary.BigEndian.Uint32(bus.RAM[0x1450:]),
			binary.BigEndian.Uint32(bus.RAM[0x17B4:]), binary.BigEndian.Uint32(bus.RAM[0x17BC:]), binary.BigEndian.Uint32(bus.RAM[0x17C4:]), binary.BigEndian.Uint32(bus.RAM[0x17CC:]), binary.BigEndian.Uint32(bus.RAM[0x17D4:]))
		printStoreImmRefs(bus, 0x224E, 16)
		printStoreImmRefs(bus, 0x1E70, 16)
		printStoreImmRefs(bus, 0x1E74, 16)
		printJALRefs(bus, 0x802C7464, 16)
		printJALRefs(bus, 0x802C52A4, 16)
		printJALRefs(bus, 0x802B417C, 32)
		printJALRefs(bus, 0x802C5EC0, 16)
		printJALRefs(bus, 0x802C5E80, 16)
		printJALRefs(bus, 0x802C5E90, 16)
		printJALRefs(bus, 0x802C5EA0, 16)
		printJALRefs(bus, 0x802C5EB0, 16)
		printJALRefs(bus, 0x802C5ED0, 16)
		printJALRefs(bus, 0x802C5EE0, 16)
		printJALRefs(bus, 0x80222E58, 32)
		printJALRefs(bus, 0x80222E68, 32)
		printLoadImmRefs(bus, 0xA0000400, 32)
		printAccessImmRefs(bus, 0x0188, 32)
		printAccessImmRefs(bus, 0x018C, 32)
		printAccessImmRefs(bus, 0x0190, 32)
		printCodeRange(bus, 0x80222E20, 0x80223180, "device base func")
		printCodeRange(bus, 0x802C5E80, 0x802C5F50, "event push func")
		printLoadImmRefs(bus, 0x80234F20, 16)
		printLoadImmRefs(bus, 0x802B4480, 16)
		printCodeRange(bus, 0x80234F20, 0x80234F80, "gate queue caller")
		printCodeRange(bus, 0x802B4460, 0x802B44C0, "gate queue caller")
		printCodeRange(bus, 0x802C5240, 0x802C5320, "gate flags func")
		printCodeRange(bus, 0x802C66F0, 0x802C6850, "gate caller")
		printCodeRange(bus, 0x802A9BC0, 0x802A9C30, "gate cond")
		printCodeRange(bus, 0x802C72D0, 0x802C7500, "gate setter")
		ptr := binary.BigEndian.Uint32(bus.RAM[0x1C90:])
		fmt.Printf("Final parser ptr=0x%08X around=%q\n", ptr, previewRAMString(bus, (ptr-96)&0x1FFFFFFF, 240))
		printRangeSummary(bus, 0x00400000, 0x00100000, "final video-ish 0x80400000")
		dumpRAMRange(bus, 0x00400000, 0x100000, "debug_video_80400000_final.bin")
		writeVideoFrameCandidates(bus)
		if os.Getenv("WEBTV_VIDEO") != "0" {
			if err := showVideoFrame(bus); err != nil {
				fmt.Fprintf(os.Stderr, "SDL video: %v\n", err)
			}
		}
		writeDebugFrameCandidates(bus, 0x00400000, 0x00100000)
		dumpRAMRange(bus, 0x0022A000, 0x900, "debug_8022a000.bin")
		dumpRAMRange(bus, 0x002AD000, 0x700, "debug_802ad000.bin")
		dumpRAMRange(bus, 0x002B4000, 0x900, "debug_802b4000.bin")
		dumpRAMRange(bus, 0x002C6600, 0x600, "debug_802c6600.bin")
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
	fmt.Printf("Modem stage=%d reads=%d writes=%d rx=%d\n", bus.ModemStage, bus.ModemReadCnt, bus.ModemWriteCnt, len(bus.ModemRX))
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

func printSchedulerNode(bus *Bus, ptr uint32, label string) {
	p := ptr & 0x1FFFFFFF
	if p+0x23 >= uint32(len(bus.RAM)) {
		fmt.Printf("sched %s ptr=0x%08X out-of-range\n", label, ptr)
		return
	}
	fmt.Printf("sched %s ptr=0x%08X +00=0x%08X +04=0x%08X +08=0x%08X +0C=0x%08X +10=%d +14=0x%08X +18=0x%08X +1C=0x%08X +20=0x%08X\n",
		label, ptr,
		binary.BigEndian.Uint32(bus.RAM[p:]),
		binary.BigEndian.Uint32(bus.RAM[p+4:]),
		binary.BigEndian.Uint32(bus.RAM[p+8:]),
		binary.BigEndian.Uint32(bus.RAM[p+12:]),
		binary.BigEndian.Uint32(bus.RAM[p+16:]),
		binary.BigEndian.Uint32(bus.RAM[p+20:]),
		binary.BigEndian.Uint32(bus.RAM[p+24:]),
		binary.BigEndian.Uint32(bus.RAM[p+28:]),
		binary.BigEndian.Uint32(bus.RAM[p+32:]))
}

func printSchedulerChain(bus *Bus, ptr uint32, label string) {
	for i := 0; i < 6 && ptr != 0; i++ {
		p := ptr & 0x1FFFFFFF
		if p+0x23 >= uint32(len(bus.RAM)) {
			fmt.Printf("sched %s[%d] ptr=0x%08X out-of-range\n", label, i, ptr)
			return
		}
		next := binary.BigEndian.Uint32(bus.RAM[p:])
		fmt.Printf("sched %s[%d] ptr=0x%08X cb=0x%08X type=0x%08X tick=%d next=0x%08X data=0x%08X flags=0x%08X\n",
			label, i, ptr, binary.BigEndian.Uint32(bus.RAM[p+8:]), binary.BigEndian.Uint32(bus.RAM[p+12:]), binary.BigEndian.Uint32(bus.RAM[p+16:]), next, binary.BigEndian.Uint32(bus.RAM[p+4:]), binary.BigEndian.Uint32(bus.RAM[p+32:]))
		ptr = next
	}
}

func printEventCallbacks(bus *Bus, table uint32, size uint32) {
	printed := 0
	for off := uint32(0); off+0x17 < size && table+off+0x17 < uint32(len(bus.RAM)); off += 4 {
		cb := binary.BigEndian.Uint32(bus.RAM[table+off+0x14:])
		if cb == 0 {
			continue
		}
		fmt.Printf("Final event cb off=0x%04X cb=0x%08X word0=0x%08X\n", off, cb, binary.BigEndian.Uint32(bus.RAM[table+off:]))
		printed++
		if printed >= 16 {
			return
		}
	}
}

func printListChain(bus *Bus, ptr uint32, label string, limit int) {
	p := ptr & 0x1FFFFFFF
	if p+7 >= uint32(len(bus.RAM)) {
		fmt.Printf("%s root=0x%08X out-of-range\n", label, ptr)
		return
	}
	head := binary.BigEndian.Uint32(bus.RAM[p:])
	meta := binary.BigEndian.Uint32(bus.RAM[p+4:])
	fmt.Printf("%s root=0x%08X head=0x%08X meta=0x%08X\n", label, ptr, head, meta)
	for i := 0; i < limit && head != 0; i++ {
		np := head & 0x1FFFFFFF
		if np+11 >= uint32(len(bus.RAM)) {
			fmt.Printf("%s node[%d] ptr=0x%08X out-of-range\n", label, i, head)
			return
		}
		next := binary.BigEndian.Uint32(bus.RAM[np:])
		key := binary.BigEndian.Uint32(bus.RAM[np+4:])
		val := binary.BigEndian.Uint32(bus.RAM[np+8:])
		fmt.Printf("%s node[%d] ptr=0x%08X next=0x%08X key=0x%08X val=0x%08X\n", label, i, head, next, key, val)
		head = next
	}
}

func printStoreImmRefs(bus *Bus, imm uint32, limit int) {
	printed := 0
	for off := uint32(0x200000); off+3 < RAM_SIZE; off += 4 {
		w := binary.BigEndian.Uint32(bus.RAM[off:])
		op := w >> 26
		if w&0xFFFF == imm && (op == 0x28 || op == 0x29 || op == 0x2B) {
			fmt.Printf("Final store imm 0x%04X at PC=0x%08X inst=0x%08X\n", imm, 0x80000000|off, w)
			printed++
			if printed >= limit {
				return
			}
		}
	}
}

func printAccessImmRefs(bus *Bus, imm uint32, limit int) {
	printed := 0
	for off := uint32(0x200000); off+3 < RAM_SIZE; off += 4 {
		w := binary.BigEndian.Uint32(bus.RAM[off:])
		op := w >> 26
		if w&0xFFFF == imm && (op == 0x20 || op == 0x21 || op == 0x23 || op == 0x24 || op == 0x25 || op == 0x28 || op == 0x29 || op == 0x2B) {
			fmt.Printf("Final access imm 0x%04X at PC=0x%08X op=0x%02X rs=%d rt=%d inst=0x%08X\n", imm, 0x80000000|off, op, (w>>21)&0x1F, (w>>16)&0x1F, w)
			printed++
			if printed >= limit {
				return
			}
		}
	}
}

func printJALRefs(bus *Bus, target uint32, limit int) {
	want := (target >> 2) & 0x03FFFFFF
	printed := 0
	for off := uint32(0x200000); off+3 < RAM_SIZE; off += 4 {
		w := binary.BigEndian.Uint32(bus.RAM[off:])
		if w>>26 == 3 && w&0x03FFFFFF == want {
			fmt.Printf("Final JAL ref target=0x%08X at PC=0x%08X inst=0x%08X\n", target, 0x80000000|off, w)
			printed++
			if printed >= limit {
				return
			}
		}
	}
}

func printLoadImmRefs(bus *Bus, target uint32, limit int) {
	printed := 0
	hi := (target >> 16) & 0xFFFF
	lo := target & 0xFFFF
	for off := uint32(0x200000); off+7 < RAM_SIZE; off += 4 {
		w0 := binary.BigEndian.Uint32(bus.RAM[off:])
		w1 := binary.BigEndian.Uint32(bus.RAM[off+4:])
		if w0>>26 != 0x0F || w0&0xFFFF != hi {
			continue
		}
		rt := (w0 >> 16) & 0x1F
		if ((w1>>26 == 0x0D || w1>>26 == 0x09) && ((w1>>21)&0x1F) == rt && ((w1>>16)&0x1F) == rt && w1&0xFFFF == lo) || (lo == 0 && w1 == 0) {
			fmt.Printf("Final load imm ref target=0x%08X at PC=0x%08X inst0=0x%08X inst1=0x%08X\n", target, 0x80000000|off, w0, w1)
			printed++
			if printed >= limit {
				return
			}
		}
	}
}

func printCodeRange(bus *Bus, start uint32, end uint32, label string) {
	for addr := start; addr < end; addr += 4 {
		fmt.Printf("%s 0x%08X 0x%08X\n", label, addr, bus.Read32(addr))
	}
}

func writeDebugFrameCandidates(bus *Bus, base uint32, size uint32) {
	if base+size > uint32(len(bus.RAM)) {
		return
	}
	first := uint32(0xFFFFFFFF)
	for i := uint32(0); i < size; i++ {
		if bus.RAM[base+i] != 0 {
			first = i
			break
		}
	}
	if first == 0xFFFFFFFF {
		return
	}
	if first > 0xE0000 {
		first = 0xE0000
	}
	writeGray4PNG(bus.RAM[base+first:], 640, 400, fmt.Sprintf("debug_frame_%08X_4bpp_640x400.png", base+first))
	writeGray8PNG(bus.RAM[base+first:], 512, 256, fmt.Sprintf("debug_frame_%08X_8bpp_512x256.png", base+first))
	fmt.Printf("Debug frames written from RAM 0x%08X\n", base+first)
}

func writeVideoFrameCandidates(bus *Bus) {
	base := bus.Asic.VIDNStart & 0x1FFFFFFF
	if base >= uint32(len(bus.RAM)) {
		return
	}
	data := bus.RAM[base:]
	writeGray4PNG(data, 640, 400, fmt.Sprintf("debug_vid_nstart_%08X_4bpp_640x400.png", base))
	writeGray8PNG(data, 512, 256, fmt.Sprintf("debug_vid_nstart_%08X_8bpp_512x256.png", base))
	writeRGB565PNG(data, 640, 400, fmt.Sprintf("debug_vid_nstart_%08X_rgb565_640x400.png", base))
	writeYUY2PNG(data, 640, 400, fmt.Sprintf("debug_vid_nstart_%08X_yuy2_640x400.png", base))
	writeUYVYPNG(data, 640, 400, fmt.Sprintf("debug_vid_nstart_%08X_uyvy_640x400.png", base))
	fmt.Printf("Video frame candidates written from VID_NSTART RAM 0x%08X\n", base)
}

func writeGray4PNG(data []byte, w int, h int, path string) {
	if len(data) < w*h/2 {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			b := data[(y*w+x)/2]
			v := b & 0x0F
			if x&1 == 0 {
				v = b >> 4
			}
			c := uint8(v * 17)
			img.SetRGBA(x, y, color.RGBA{c, c, c, 0xFF})
		}
	}
	writePNG(path, img)
}

func writeGray8PNG(data []byte, w int, h int, path string) {
	if len(data) < w*h {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := data[y*w+x]
			img.SetRGBA(x, y, color.RGBA{c, c, c, 0xFF})
		}
	}
	writePNG(path, img)
}

func writeRGB565PNG(data []byte, w int, h int, path string) {
	if len(data) < w*h*2 {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := binary.BigEndian.Uint16(data[(y*w+x)*2:])
			r := uint8(uint32((v>>11)&0x1F) * 255 / 31)
			g := uint8(uint32((v>>5)&0x3F) * 255 / 63)
			b := uint8(uint32(v&0x1F) * 255 / 31)
			img.SetRGBA(x, y, color.RGBA{r, g, b, 0xFF})
		}
	}
	writePNG(path, img)
}

func writeYUY2PNG(data []byte, w int, h int, path string) {
	writeYUV422PNG(data, w, h, path, false)
}

func writeUYVYPNG(data []byte, w int, h int, path string) {
	writeYUV422PNG(data, w, h, path, true)
}

func writeYUV422PNG(data []byte, w int, h int, path string, uyvy bool) {
	if len(data) < w*h*2 || w&1 != 0 {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x += 2 {
			off := (y*w + x) * 2
			var y0, u, y1, v uint8
			if uyvy {
				u, y0, v, y1 = data[off], data[off+1], data[off+2], data[off+3]
			} else {
				y0, u, y1, v = data[off], data[off+1], data[off+2], data[off+3]
			}
			r, g, b := yuvToRGB(y0, u, v)
			img.SetRGBA(x, y, color.RGBA{r, g, b, 0xFF})
			r, g, b = yuvToRGB(y1, u, v)
			img.SetRGBA(x+1, y, color.RGBA{r, g, b, 0xFF})
		}
	}
	writePNG(path, img)
}

func yuvToRGB(y, u, v uint8) (uint8, uint8, uint8) {
	c := int(y) - 16
	d := int(u) - 128
	e := int(v) - 128
	return clampByte((298*c + 409*e + 128) >> 8), clampByte((298*c - 100*d - 208*e + 128) >> 8), clampByte((298*c + 516*d + 128) >> 8)
}

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func writePNG(path string, img image.Image) {
	path = filepath.Join("captures", filepath.Base(path))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "png directory %s: %v\n", filepath.Dir(path), err)
		return
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "png %s: %v\n", path, err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintf(os.Stderr, "png %s: %v\n", path, err)
	}
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
	path = filepath.Join(".tmp", "dumps", filepath.Base(path))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "dump directory %s: %v\n", filepath.Dir(path), err)
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

func previewPhysString(bus *Bus, off uint32, limit int) string {
	out := make([]byte, 0, limit)
	for i := 0; i < limit; i++ {
		p := off + uint32(i)
		var b byte
		switch {
		case p < uint32(len(bus.RAM)):
			b = bus.RAM[p]
		case p >= ROM_BASE && p < ROM_BASE+uint32(len(bus.ROM)):
			b = bus.ROM[p-ROM_BASE]
		default:
			return string(out)
		}
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
