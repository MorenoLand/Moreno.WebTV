package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

const (
	RAM_SIZE     = 8 * 1024 * 1024
	RAM_APERTURE = 64 * 1024 * 1024
	ROM_MAX      = 16 * 1024 * 1024
	ROM_BASE     = 0x1F000000
	GLOBAL_MASK  = 0x1FFFFFFF
)

type Bus struct {
	RAM                      [RAM_SIZE]byte
	ROM                      [ROM_MAX]byte
	ROMSz                    uint32
	Asic                     *SoloASIC
	IDE                      *IDEController
	CPU                      *CPU
	AsicReadCnt              [0x10000]uint64
	Tracking                 bool
	PollCnt                  map[uint32]uint64
	RIOReadCnt               uint64
	RIOWriteCnt              uint64
	ModemReadCnt             uint64
	ModemWriteCnt            uint64
	ModemPostCarrierWrites   uint64
	ModemPostCarrierTXBytes  uint64
	ModemRX                  []byte
	ModemTXLine              []byte
	ModemStage               int
	ModemStageAt             uint64
	ModemPPPBytes            int
	ModemLCPQueued           bool
	ModemWireRX              []byte
	ModemNextRXAt            uint64
	SyntheticPPP             bool
	CompatShims              bool
	ModemIER                 uint8
	ModemLCR                 uint8
	ModemMCR                 uint8
	ModemMSR                 uint8
	ModemDLL                 uint8
	ModemDLM                 uint8
	ModemReadyAt             uint64
	ModemLink                *modemTCPLink
	SkipNullCleanup          bool
	TraceNullFlow            bool
	TraceNullCount           uint32
	TraceDeviceField         bool
	TraceFactoryCount        uint32
	TraceRegistryInitCount   uint32
	TraceFatalCount          uint32
	TraceCallbackPrepCount   uint32
	TraceCallbackOriginCount uint32
	TraceTimerEventCount     uint32
	TracePage                bool
	TracePageCount           uint32
	TraceWebTVPort           bool
	WebTVPortTraceCount      uint32
	TraceRIOControl          bool
	RIOControlTraceCount     uint32
	TraceModemTX             bool
	ModemTXTraceCount        uint32
	TraceDeviceObject        bool
	DeviceObjectTraceCount   uint32
	DeviceObjectWriteCount   uint32
	ModemGlobalTraceCount    uint32
	TracePacketQueue         bool
	PacketQueueTraceCount    uint32
}

func NewBus() *Bus {
	b := &Bus{SkipNullCleanup: os.Getenv("WEBTV_SKIP_NULL_CLEANUP") == "1", TraceNullFlow: os.Getenv("WEBTV_TRACE_NULL_FLOW") == "1", TraceDeviceField: os.Getenv("WEBTV_TRACE_DEVICE_FIELD") == "1", TracePage: os.Getenv("WEBTV_TRACE_PAGE") == "1", TraceWebTVPort: os.Getenv("WEBTV_TRACE_WTV_PORT") == "1", TraceRIOControl: os.Getenv("WEBTV_TRACE_RIO_CONTROL") == "1", TraceModemTX: os.Getenv("WEBTV_TRACE_MODEM_TX") == "1", TraceDeviceObject: os.Getenv("WEBTV_TRACE_DEVICE_OBJECT") == "1", TracePacketQueue: os.Getenv("WEBTV_TRACE_PACKET_QUEUE") == "1", SyntheticPPP: os.Getenv("WEBTV_SYNTHETIC_PPP") == "1", CompatShims: os.Getenv("WEBTV_COMPAT_SHIMS") == "1"}
	b.Asic = NewSoloASIC()
	b.IDE = NewIDEController(b.Asic)
	b.PollCnt = make(map[uint32]uint64)
	return b
}

func (b *Bus) LoadBootROM(bank0, bank1 []byte) {
	size := len(bank0)
	if size > ROM_MAX {
		size = ROM_MAX
	}
	for i := 0; i < size/2; i++ {
		upper := binary.BigEndian.Uint16(bank0[i*2:])
		lower := binary.BigEndian.Uint16(bank1[i*2:])
		offset := i * 4
		if offset+4 <= ROM_MAX {
			binary.BigEndian.PutUint16(b.ROM[0xC00000+offset:], upper)
			binary.BigEndian.PutUint16(b.ROM[0xC00000+offset+2:], lower)
		}
	}
	b.ROMSz = uint32(size) * 2
}

func (b *Bus) phys(addr uint32) uint32 {
	if addr >= 0x80000000 && addr < 0xC0000000 {
		return addr & 0x1FFFFFFF
	}
	if b.CPU != nil && b.CPU.COP0[Status]&0x04 != 0 { // ERL set — all addrs unmapped
		return addr & 0x1FFFFFFF
	}
	if b.CPU != nil && b.CPU.COP0[Status]&0x02 != 0 && b.CPU.COP0[Status]&0x00400000 == 0 && addr < RAM_APERTURE {
		return addr
	}
	if b.CPU != nil {
		paddr, ok := b.CPU.Translate(addr)
		if ok {
			return paddr
		}
		// TLB miss — raise exception
		b.CPU.RaiseTLBMiss(addr)
		return 0xFFFFFFFF
	}
	return addr & GLOBAL_MASK
}

func ramMirror(p uint32, width uint32) (uint32, bool) {
	if width <= RAM_APERTURE && p <= RAM_APERTURE-width {
		return p % RAM_SIZE, true
	}
	return 0, false
}

const webTVPortProbeAddress uint32 = 0x04FFFFF3
const webTVPortProbeWordAddress uint32 = 0x04FFFFF0
const webTVPortBase uint32 = 0x04F00000
const webTVPortSize uint32 = 0x18
const modemCharCycles uint64 = 14500

func (b *Bus) traceWebTVPort(op string, p, width, value uint32) {
	if !b.TraceWebTVPort || b.WebTVPortTraceCount >= 128 || !((p >= webTVPortBase && p < webTVPortBase+webTVPortSize && width <= webTVPortBase+webTVPortSize-p) || (p >= webTVPortProbeWordAddress && p < webTVPortProbeWordAddress+4 && width <= webTVPortProbeWordAddress+4-p)) || b.CPU == nil {
		return
	}
	b.WebTVPortTraceCount++
	fmt.Printf("WebTV port %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X width=%d val=0x%08X v0=0x%08X a0=0x%08X a1=0x%08X ra=0x%08X\n", op, b.WebTVPortTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), p, width, value, uint32(b.CPU.Regs[2]), uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) traceRIOControl(op string, off, width, value uint32) {
	if !b.TraceRIOControl || b.RIOControlTraceCount >= 64 || b.CPU == nil {
		return
	}
	switch off {
	case 0x100C, 0x101C, 0x1020:
	default:
		return
	}
	b.RIOControlTraceCount++
	fmt.Printf("RIO control %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X off=0x%04X width=%d val=0x%08X a0=0x%08X a1=0x%08X ra=0x%08X\n", op, b.RIOControlTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), off, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) traceDeviceObjectRead(rp, value uint32) {
	if !b.TraceDeviceObject || b.DeviceObjectTraceCount >= 128 || (rp != 0x400 && rp != 0x588) || b.CPU == nil {
		return
	}
	b.DeviceObjectTraceCount++
	fmt.Printf("device object read #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X phys=0x%08X val=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", b.DeviceObjectTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), rp, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[17]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) traceDeviceObjectWrite(rp, width, value uint32) {
	if !b.TraceDeviceObject || b.DeviceObjectWriteCount >= 128 || rp >= 0x58C || rp+width <= 0x588 || b.CPU == nil {
		return
	}
	b.DeviceObjectWriteCount++
	fmt.Printf("device object write #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X phys=0x%08X width=%d val=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", b.DeviceObjectWriteCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), rp, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[17]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) traceModemGlobal(op string, rp, width, value uint32) {
	if !b.TraceDeviceObject || b.ModemGlobalTraceCount >= 128 || rp >= 0x1CFC || rp+width <= 0x1CF8 || b.CPU == nil {
		return
	}
	b.ModemGlobalTraceCount++
	fmt.Printf("modem global %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X phys=0x%08X width=%d val=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", op, b.ModemGlobalTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), rp, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[17]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) tracePacketQueue(op string, rp, width, value uint32) {
	if !b.TracePacketQueue || b.PacketQueueTraceCount >= 128 || rp >= 0x1A4C || rp+width <= 0x1A3C || b.CPU == nil {
		return
	}
	b.PacketQueueTraceCount++
	fmt.Printf("packet queue %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X phys=0x%08X width=%d val=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", op, b.PacketQueueTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), rp, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[17]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) Read8(addr uint32) uint8 {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 1); ok {
		v := b.RAM[rp]
		b.traceModemGlobal("read", rp, 1, uint32(v))
		b.tracePacketQueue("read", rp, 1, uint32(v))
		return v
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX) {
		b.tracePageRead(p, 1)
		off := p - ROM_BASE
		return b.ROM[off]
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		return b.modemRead8(p - 0x1E000000)
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		v := b.Asic.Read(off)
		b.traceRIOControl("read", off, 1, v&0xFF)
		return uint8(v)
	}
	b.traceWebTVPort("read", p, 1, 0xFF)
	return 0xFF
}

func (b *Bus) Read16(addr uint32) uint16 {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 2); ok {
		v := binary.BigEndian.Uint16(b.RAM[rp:])
		b.traceModemGlobal("read", rp, 2, uint32(v))
		b.tracePacketQueue("read", rp, 2, uint32(v))
		return v
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX)-1 {
		b.tracePageRead(p, 2)
		off := p - ROM_BASE
		return binary.BigEndian.Uint16(b.ROM[off:])
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		return uint16(b.modemRead8(p - 0x1E000000))
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		v := b.Asic.Read(off)
		b.traceRIOControl("read", off, 2, v&0xFFFF)
		return uint16(v)
	}
	if p >= 0x1E400000 && p <= 0x1E80001F {
		return uint16(b.IDE.Read32(p - 0x1E400000))
	}
	b.traceWebTVPort("read", p, 2, 0xFFFF)
	return 0xFFFF
}

func (b *Bus) Read32(addr uint32) uint32 {
	p := b.phys(addr)
	if p >= 0x04000000 && p < 0x04010000 {
		if b.Tracking {
			b.AsicReadCnt[p&0xFFFF]++
		}
		off := p & 0xFFFF
		v := b.Asic.Read(off)
		b.traceRIOControl("read", off, 4, v)
		if off == 0x90A0 && b.CPU != nil {
			b.RIOReadCnt++
			if b.RIOReadCnt <= 32 || b.RIOReadCnt%100000 == 0 {
				fmt.Printf("RIO read #%d step=%d PC=0x%08X execPC=0x%08X off=0x%04X val=0x%08X cycles=0x%016X readyAt=0x%016X ptr=0x%08X\n",
					b.RIOReadCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, off, v, b.Asic.Cycles, b.Asic.RIOPollReadyAt, binary.BigEndian.Uint32(b.RAM[0x1C90:]))
			}
		}
		return v
	}
	if p >= 0x1E400000 && p <= 0x1E80001F {
		return b.IDE.Read32(p - 0x1E400000)
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		return uint32(b.modemRead8(p - 0x1E000000))
	}
	if rp, ok := ramMirror(p, 4); ok {
		v := binary.BigEndian.Uint32(b.RAM[rp:])
		b.traceDeviceObjectRead(rp, v)
		b.traceModemGlobal("read", rp, 4, v)
		b.tracePacketQueue("read", rp, 4, v)
		if b.Tracking && ((rp >= 0x710000 && rp <= 0x71FFFF) || (rp >= 0x7F0000 && rp <= 0x7FFFFF)) {
			b.PollCnt[rp]++
		}
		return v
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX)-3 {
		b.tracePageRead(p, 4)
		off := p - ROM_BASE
		return binary.BigEndian.Uint32(b.ROM[off:])
	}
	b.traceWebTVPort("read", p, 4, 0xFFFFFFFF)
	return 0xFFFFFFFF
}

func (b *Bus) tracePageRead(p, width uint32) {
	if !b.TracePage || b.CPU == nil || b.TracePageCount >= 64 {
		return
	}
	ranges := [...]struct {
		start uint32
		size  uint32
		name  string
	}{{0x1FD23000, 600, "RemovePrinterAdapter.html"}, {0x1FD239F4, 420, "Startup.html"}, {0x1FD2FC34, 505, "Connecting.html"}, {0x1FD17554, 14384, "ConnectingTo.gif"}, {0x1FD1AD88, 4890, "CityRoad.gif"}}
	for _, page := range ranges {
		if p < page.start+page.size && p+width > page.start {
			b.TracePageCount++
			fmt.Printf("page ROM read %02d step=%d pc=0x%08X execPC=0x%08X name=%s addr=0x%08X width=%d ra=0x%08X\n", b.TracePageCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, page.name, p, width, uint32(b.CPU.Regs[31]))
			return
		}
	}
}

func (b *Bus) Write8(addr, val uint32) {
	p := b.phys(addr)
	if b.TraceDeviceField && p >= 0x1E20 && p < 0x1E2C && b.CPU != nil {
		fmt.Printf("decoder-registry write8 step=%d pc=0x%08X inst=0x%08X addr=0x%08X val=0x%02X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), p, val&0xFF)
	}
	if b.TraceDeviceField && p == 0xEADB4 && b.CPU != nil {
		fmt.Printf("device-field write8 step=%d pc=0x%08X inst=0x%08X val=0x%02X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), val&0xFF)
	}
	if b.TraceDeviceField && p == 0x2658 && b.CPU != nil {
		fmt.Printf("factory-global write8 step=%d pc=0x%08X inst=0x%08X val=0x%02X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), val&0xFF)
	}
	if rp, ok := ramMirror(p, 1); ok {
		b.traceDeviceObjectWrite(rp, 1, val&0xFF)
		b.traceModemGlobal("write", rp, 1, val&0xFF)
		b.tracePacketQueue("write", rp, 1, val&0xFF)
		if b.Tracking && ((rp >= 0x330 && rp <= 0x590) || (rp >= 0x288 && rp <= 0x28B) || (rp >= 0x101C && rp <= 0x101F) || (rp >= 0x16A0 && rp <= 0x16A3) || (rp >= 0x17A0 && rp <= 0x17A3) || (rp >= 0x1904 && rp <= 0x1909) || (rp >= 0x1CF0 && rp <= 0x1CF7) || (rp >= 0x2240 && rp <= 0x224E) || (rp >= 0x2730 && rp <= 0x2733) || (rp >= 0x7E930 && rp <= 0x7E960) || (rp >= 0x80A88 && rp <= 0x80A98) || (rp >= 0xA510 && rp <= 0xA690) || rp == 0x1E74) && b.CPU != nil {
			fmt.Printf("RAM watch8 PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X phys=0x%08X val=0x%02X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, rp, val&0xFF)
		}
		b.RAM[rp] = byte(val)
		return
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		b.traceRIOControl("write", off, 1, val&0xFF)
		if off == 0x90A8 && b.CPU != nil {
			b.RIOWriteCnt++
			if b.RIOWriteCnt <= 64 || b.RIOWriteCnt%1000 == 0 {
				fmt.Printf("RIO write8 #%d step=%d PC=0x%08X execPC=0x%08X off=0x%04X val=0x%02X cycles=0x%016X ptr=0x%08X\n",
					b.RIOWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, p&0xFFFF, val&0xFF, b.Asic.Cycles, binary.BigEndian.Uint32(b.RAM[0x1C90:]))
			}
		}
		b.Asic.Write(off, val)
		b.updateModemIRQ()
		return
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		b.modemWrite(p-0x1E000000, val, 1)
		return
	}
	b.traceWebTVPort("write", p, 1, val)
}

func (b *Bus) Write16(addr, val uint32) {
	p := b.phys(addr)
	if b.TraceDeviceField && p < 0x1E2C && p+1 >= 0x1E20 && b.CPU != nil {
		fmt.Printf("decoder-registry write16 step=%d pc=0x%08X inst=0x%08X addr=0x%08X val=0x%04X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), p, val&0xFFFF)
	}
	if b.TraceDeviceField && p <= 0xEADB4 && p+1 >= 0xEADB4 && b.CPU != nil {
		fmt.Printf("device-field write16 step=%d pc=0x%08X inst=0x%08X val=0x%04X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), val&0xFFFF)
	}
	if b.TraceDeviceField && p <= 0x2658 && p+1 >= 0x2658 && b.CPU != nil {
		fmt.Printf("factory-global write16 step=%d pc=0x%08X inst=0x%08X addr=0x%08X val=0x%04X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), p, val&0xFFFF)
	}
	if rp, ok := ramMirror(p, 2); ok {
		b.traceDeviceObjectWrite(rp, 2, val&0xFFFF)
		b.traceModemGlobal("write", rp, 2, val&0xFFFF)
		b.tracePacketQueue("write", rp, 2, val&0xFFFF)
		if b.Tracking && ((rp <= 0x590 && rp+1 >= 0x330) || (rp <= 0x28B && rp+1 >= 0x288) || (rp <= 0x101F && rp+1 >= 0x101C) || (rp <= 0x16A3 && rp+1 >= 0x16A0) || (rp <= 0x17A3 && rp+1 >= 0x17A0) || (rp <= 0x1909 && rp+1 >= 0x1904) || (rp <= 0x1CF7 && rp+1 >= 0x1CF0) || (rp <= 0x224E && rp+1 >= 0x2240) || (rp <= 0x2733 && rp+1 >= 0x2730) || (rp <= 0x7E960 && rp+1 >= 0x7E930) || (rp <= 0x80A98 && rp+1 >= 0x80A88) || (rp <= 0xA690 && rp+1 >= 0xA510) || (rp <= 0x1E74 && rp+1 >= 0x1E74)) && b.CPU != nil {
			fmt.Printf("RAM watch16 PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X phys=0x%08X val=0x%04X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, rp, val&0xFFFF)
		}
		binary.BigEndian.PutUint16(b.RAM[rp:], uint16(val))
		return
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		b.traceRIOControl("write", off, 2, val&0xFFFF)
		if off == 0x90A8 && b.CPU != nil {
			b.RIOWriteCnt++
			if b.RIOWriteCnt <= 64 || b.RIOWriteCnt%1000 == 0 {
				fmt.Printf("RIO write16 #%d step=%d PC=0x%08X execPC=0x%08X off=0x%04X val=0x%04X cycles=0x%016X ptr=0x%08X\n",
					b.RIOWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, p&0xFFFF, val&0xFFFF, b.Asic.Cycles, binary.BigEndian.Uint32(b.RAM[0x1C90:]))
			}
		}
		b.Asic.Write(off, val)
		b.updateModemIRQ()
		return
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		b.modemWrite(p-0x1E000000, val, 2)
		return
	}
	b.traceWebTVPort("write", p, 2, val)
}

func (b *Bus) Write32(addr, val uint32) {
	p := b.phys(addr)
	if b.TraceDeviceField && p < 0x1E2C && p+3 >= 0x1E20 && b.CPU != nil {
		fmt.Printf("decoder-registry write32 step=%d pc=0x%08X inst=0x%08X addr=0x%08X val=0x%08X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), p, val)
	}
	if b.TraceDeviceField && p <= 0xEADB4 && p+3 >= 0xEADB4 && b.CPU != nil {
		fmt.Printf("device-field write32 step=%d pc=0x%08X inst=0x%08X val=0x%08X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), val)
	}
	if b.TraceDeviceField && p <= 0x2658 && p+3 >= 0x2658 && b.CPU != nil {
		fmt.Printf("factory-global write32 step=%d pc=0x%08X inst=0x%08X addr=0x%08X val=0x%08X a0=0x%08X s0=0x%08X ra=0x%08X\n", b.CPU.Steps, b.CPU.excPC, b.Read32(b.CPU.excPC), p, val, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[31]))
	}
	if p >= 0x1E400000 && p <= 0x1E80001F {
		b.IDE.Write32(p-0x1E400000, val)
		return
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		b.modemWrite(p-0x1E000000, val, 4)
		return
	}
	if rp, ok := ramMirror(p, 4); ok {
		b.traceDeviceObjectWrite(rp, 4, val)
		b.traceModemGlobal("write", rp, 4, val)
		b.tracePacketQueue("write", rp, 4, val)
		if b.Tracking && ((rp <= 0x590 && rp+3 >= 0x330) || (rp <= 0x28B && rp+3 >= 0x288) || (rp <= 0x101F && rp+3 >= 0x101C) || (rp <= 0x16A3 && rp+3 >= 0x16A0) || (rp <= 0x17A3 && rp+3 >= 0x17A0) || (rp <= 0x1907 && rp+3 >= 0x18F0) || (rp <= 0x1CF7 && rp+3 >= 0x1CF0) || (rp <= 0x224E && rp+3 >= 0x2240) || (rp <= 0x2733 && rp+3 >= 0x2730) || (rp <= 0x7E960 && rp+3 >= 0x7E930) || (rp <= 0x80A98 && rp+3 >= 0x80A88) || (rp <= 0xA690 && rp+3 >= 0xA510) || (rp <= 0x1E74 && rp+3 >= 0x1E74) || (rp <= 0xEEDD0 && rp+3 >= 0xEEDA8) || (rp <= 0xF0B93 && rp+3 >= 0xF0B70)) && b.CPU != nil {
			fmt.Printf("RAM watch32 PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X phys=0x%08X val=0x%08X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, rp, val)
			if rp <= 0x101F && rp+3 >= 0x101C {
				fmt.Printf("  timer gate write step=%d Count=0x%08X Compare=0x%08X Cause=0x%08X Status=0x%08X\n",
					b.CPU.Steps, b.CPU.COP0[Count], b.CPU.COP0[Compare], b.CPU.COP0[Cause], b.CPU.COP0[Status])
			}
		}
		if rp == 0x000EAD98 && b.CPU != nil {
			fmt.Printf("object f4 write PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X val=0x%08X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, val)
		}
		binary.BigEndian.PutUint32(b.RAM[rp:], val)
		return
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		b.traceRIOControl("write", off, 4, val)
		if off == 0x90A8 && b.CPU != nil {
			b.RIOWriteCnt++
			if b.RIOWriteCnt <= 64 || b.RIOWriteCnt%1000 == 0 {
				fmt.Printf("RIO write32 #%d step=%d PC=0x%08X execPC=0x%08X off=0x%04X val=0x%08X cycles=0x%016X ptr=0x%08X\n",
					b.RIOWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, p&0xFFFF, val, b.Asic.Cycles, binary.BigEndian.Uint32(b.RAM[0x1C90:]))
			}
		}
		b.Asic.Write(off, val)
		b.updateModemIRQ()
		return
	}
	b.traceWebTVPort("write", p, 4, val)
}

func (b *Bus) modemRead8(off uint32) uint8 {
	b.ModemReadCnt++
	var v uint8
	switch off {
	case 0x00:
		if b.ModemLCR&0x80 != 0 {
			v = b.ModemDLL
			break
		}
		if len(b.ModemRX) == 0 {
			v = 0
			break
		}
		v = b.ModemRX[0]
		b.ModemRX = b.ModemRX[1:]
		if len(b.ModemRX) == 0 && b.ModemStage == 1 {
			b.ModemStage = 2
			b.ModemStageAt = b.ModemWriteCnt
			if b.CPU != nil {
				fmt.Printf("modem stage 1 consumed step=%d writes=%d reads=%d\n", b.CPU.Steps, b.ModemWriteCnt, b.ModemReadCnt)
			}
		}
		if len(b.ModemRX) == 0 && b.ModemStage == 3 {
			b.ModemStage = 4
			if b.CPU != nil {
				fmt.Printf("modem stage 2 consumed step=%d writes=%d reads=%d\n", b.CPU.Steps, b.ModemWriteCnt, b.ModemReadCnt)
			}
		}
		b.updateModemIRQ()
	case 0x04:
		if b.ModemLCR&0x80 != 0 {
			v = b.ModemDLM
		} else {
			v = b.ModemIER
		}
	case 0x08:
		if len(b.ModemRX) != 0 && b.ModemIER&1 != 0 {
			v = 0x04
		} else if b.ModemMSR&0x0F != 0 && b.ModemIER&8 != 0 {
			v = 0x00
		} else {
			v = 0x01
		}
	case 0x0C:
		v = b.ModemLCR
	case 0x14:
		v = 0x20
		if len(b.ModemRX) != 0 {
			v |= 0x01
		}
	case 0x18:
		v = b.ModemMSR
		b.ModemMSR &^= 0x0F
		if b.SyntheticPPP && b.CPU != nil && b.ModemStage >= 4 && b.ModemLink == nil && !b.ModemLCPQueued && v&0x80 != 0 {
			b.ModemLCPQueued = true
			frame := pppLCPConfigureRequest(1)
			b.queueModemWireRX(frame)
			if b.CPU != nil {
				fmt.Printf("synthetic PPP LCP Configure-Request queued step=%d bytes=%d\n", b.CPU.Steps, len(frame))
			}
		}
		b.updateModemIRQ()
	default:
		v = 0
	}
	if b.CPU != nil && (b.ModemReadCnt <= 128 || b.ModemStage >= 3 || b.ModemReadCnt%1000000 == 0) {
		fmt.Printf("modem read #%d step=%d PC=0x%08X execPC=0x%08X off=0x%02X val=0x%02X\n", b.ModemReadCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, off, v)
	}
	return v
}

func (b *Bus) modemWrite(off, val, width uint32) {
	b.ModemWriteCnt++
	if b.ModemStage >= 4 {
		b.ModemPostCarrierWrites++
		if b.TraceModemTX && b.ModemPostCarrierWrites <= 64 && b.CPU != nil {
			fmt.Printf("modem post-carrier write #%d step=%d PC=0x%08X execPC=0x%08X off=0x%02X width=%d val=0x%X\n", b.ModemPostCarrierWrites, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, off, width, val)
		}
	}
	if b.CPU != nil && (b.ModemWriteCnt <= 128 || b.ModemWriteCnt%1000000 == 0) {
		fmt.Printf("modem write #%d step=%d PC=0x%08X execPC=0x%08X off=0x%02X width=%d val=0x%X\n", b.ModemWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, off, width, val)
	}
	switch off {
	case 0x00:
		if b.ModemLCR&0x80 != 0 {
			b.ModemDLL = uint8(val)
			break
		}
		ch := byte(val)
		if b.ModemStage >= 4 {
			b.ModemPostCarrierTXBytes++
			if b.TraceModemTX && b.ModemTXTraceCount < 64 && b.CPU != nil {
				b.ModemTXTraceCount++
				fmt.Printf("modem post-carrier tx #%d step=%d PC=0x%08X execPC=0x%08X val=0x%02X\n", b.ModemTXTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, ch)
			}
			if b.ModemLink != nil {
				b.ModemLink.send([]byte{ch})
			}
			if ch == 0x7E {
				if b.ModemPPPBytes != 0 && b.CPU != nil {
					fmt.Printf("modem PPP frame step=%d bytes=%d\n", b.CPU.Steps, b.ModemPPPBytes)
				}
				b.ModemPPPBytes = 0
			} else {
				b.ModemPPPBytes++
			}
		}
		if ch != '\r' && ch != '\n' {
			b.ModemTXLine = append(b.ModemTXLine, ch)
		}
		if ch == '\r' || ch == '\n' {
			line := string(b.ModemTXLine)
			if b.ModemStage == 0 && strings.Contains(line, "AT**") {
				b.queueModemRX([]byte("\n\nDownload Modem Firmware .."))
				b.ModemStage = 1
				if b.CPU != nil {
					fmt.Printf("modem stage 1 queued step=%d writes=%d line=%q\n", b.CPU.Steps, b.ModemWriteCnt, line)
				}
			}
			if b.ModemStage >= 1 && b.ModemStage < 3 && strings.HasPrefix(line, "S705") {
				b.queueModemRX([]byte("."))
				b.ModemStage = 3
				if b.CPU != nil {
					fmt.Printf("modem stage 2 queued step=%d writes=%d line=%q\n", b.CPU.Steps, b.ModemWriteCnt, line)
				}
			}
			b.ModemTXLine = b.ModemTXLine[:0]
		}
	case 0x04:
		if b.ModemLCR&0x80 != 0 {
			b.ModemDLM = uint8(val)
		} else {
			b.ModemIER = uint8(val)
			b.updateModemIRQ()
		}
	case 0x08:
		if val&2 != 0 {
			b.ModemRX = b.ModemRX[:0]
		}
		b.updateModemIRQ()
	case 0x0C:
		b.ModemLCR = uint8(val)
	case 0x10:
		b.ModemMCR = uint8(val)
		old := b.ModemMSR & 0xF0
		current := old
		if b.ModemStage >= 4 && b.ModemMCR&3 == 3 {
			if current == 0 && b.ModemReadyAt == 0 {
				b.ModemReadyAt = b.Asic.Cycles + 10000
			}
		} else {
			current = 0
			b.ModemReadyAt = 0
		}
		delta := b.ModemMSR & 0x0F
		b.ModemMSR = current | delta
		if old&0x10 != current&0x10 {
			b.ModemMSR |= 0x01
		}
		if old&0x20 != current&0x20 {
			b.ModemMSR |= 0x02
		}
		if old&0x80 != current&0x80 {
			b.ModemMSR |= 0x08
		}
		b.updateModemIRQ()
	}
}

func (b *Bus) Tick(cycles uint64) {
	b.Asic.Tick(cycles)
	b.pollModemLink()
	b.releaseModemRX()
	if b.ModemReadyAt != 0 && b.Asic.Cycles >= b.ModemReadyAt {
		b.ModemReadyAt = 0
		old := b.ModemMSR & 0xF0
		b.ModemMSR = 0xB0 | b.ModemMSR&0x0F
		if old&0x10 == 0 {
			b.ModemMSR |= 0x01
		}
		if old&0x20 == 0 {
			b.ModemMSR |= 0x02
		}
		if old&0x80 == 0 {
			b.ModemMSR |= 0x08
		}
	}
	b.updateModemIRQ()
}

func (b *Bus) pollModemLink() {
	if b.ModemLink == nil || b.ModemStage < 4 {
		return
	}
	if data := b.ModemLink.poll(); len(data) != 0 {
		b.queueModemWireRX(data)
	}
}

func (b *Bus) queueModemRX(data []byte) {
	b.ModemRX = append(b.ModemRX, data...)
	b.updateModemIRQ()
}

func (b *Bus) queueModemWireRX(data []byte) {
	if len(data) == 0 {
		return
	}
	b.ModemWireRX = append(b.ModemWireRX, data...)
	if b.ModemNextRXAt == 0 || b.ModemNextRXAt <= b.Asic.Cycles {
		b.ModemNextRXAt = b.Asic.Cycles + modemCharCycles
	}
	b.releaseModemRX()
}

func (b *Bus) releaseModemRX() {
	if len(b.ModemRX) != 0 || len(b.ModemWireRX) == 0 || b.Asic.Cycles < b.ModemNextRXAt {
		return
	}
	b.ModemRX = append(b.ModemRX, b.ModemWireRX[0])
	b.ModemWireRX = b.ModemWireRX[1:]
	b.ModemNextRXAt = b.Asic.Cycles + modemCharCycles
	b.updateModemIRQ()
}

func (b *Bus) updateModemIRQ() {
	pending := len(b.ModemRX) != 0 && b.ModemIER&1 != 0 || b.ModemMSR&0x0F != 0 && b.ModemIER&8 != 0
	if pending {
		b.Asic.RIOIntStat |= 4
		b.Asic.BusIntStat |= BUS_INT_RIO
	}
}

func (b *Bus) PrintTopPollAddrs(n int) {
	type kv struct {
		Addr uint32
		Hits uint64
	}
	var sorted []kv
	for addr, hits := range b.PollCnt {
		if hits > 1000 {
			sorted = append(sorted, kv{addr, hits})
		}
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Hits > sorted[i].Hits {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if n > len(sorted) {
		n = len(sorted)
	}
	fmt.Printf("Top %d polled RAM addresses:\n", n)
	for i := 0; i < n; i++ {
		val := binary.BigEndian.Uint32(b.RAM[sorted[i].Addr:])
		fmt.Printf("  0x%08X: %d reads, value=0x%08X\n", sorted[i].Addr, sorted[i].Hits, val)
	}
}
