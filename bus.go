package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	RAM_SIZE                  = 8 * 1024 * 1024
	RAM_APERTURE              = 64 * 1024 * 1024
	ROM_MAX                   = 16 * 1024 * 1024
	ROM_BASE                  = 0x1F000000
	GLOBAL_MASK               = 0x1FFFFFFF
	modemSRAMSize             = 0x40000
	modemNoCarrierDelayCycles = 10000
)

type Bus struct {
	RAM                      [RAM_SIZE]byte
	ROM                      [ROM_MAX]byte
	ROMSz                    uint32
	Asic                     *SoloASIC
	IDE                      *IDEController
	CPU                      *CPU
	IRInput                  chan byte
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
	ModemFirmware            map[uint32]byte
	ModemFirmwareRecords     int
	ModemFirmwareBytes       int
	ModemFirmwareReady       bool
	ForceModemDeviceStatus   bool
	PersistModemDeviceStatus bool
	ModemDeviceStatus        uint32
	PersistModemDeviceFields bool
	ModemDeviceField18C      uint32
	ModemDeviceField190      uint32
	ModemPPPBytes            int
	ModemPPPFrames           uint64
	ModemLastPPPProtocol     uint16
	ModemLastPPPInfo         []byte
	ModemLCPQueued           bool
	ModemWireRX              []byte
	ModemNextRXAt            uint64
	ModemPPPDecoder          pppDecoder
	SyntheticPPP             bool
	TraceModemPPP            bool
	ModemSRAM                [modemSRAMSize]byte
	CompatShims              bool
	ModemIER                 uint8
	ModemLCR                 uint8
	ModemMCR                 uint8
	ModemMSR                 uint8
	ModemDLL                 uint8
	ModemDLM                 uint8
	ModemBackdoor            bool
	ModemReadyAt             uint64
	ModemNoCarrierAt         uint64
	ModemNoCarrierQueued     bool
	ModemNoCarrier           bool
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
	TraceVideoASIC           bool
	VideoASICTraceCount      uint32
	VideoTableReadCount      uint32
	TraceVideoFrame          bool
	VideoFrameWriteCount     uint32
	TraceModemTX             bool
	ModemTXTraceCount        uint32
	TraceDeviceObject        bool
	DeviceObjectTraceCount   uint32
	DeviceObjectWriteCount   uint32
	TraceDevicePage          bool
	DevicePageTraceCount     uint32
	ModemGlobalTraceCount    uint32
	TracePacketQueue         bool
	PacketQueueTraceCount    uint32
	ProbeModemQueue          bool
	ModemQueueProbePtr       uint32
	ModemQueueProbeMask      uint32
	ModemQueueProbeSeeded    bool
}

func NewBus() *Bus {
	modemDeviceStatus, forceModemDeviceStatus := modemDeviceStatusFromEnv()
	modemDeviceField18C, modemDeviceField190, persistModemDeviceFields := modemDeviceFieldsFromEnv()
	modemQueueProbePtr, modemQueueProbeMask, probeModemQueue := modemQueueProbeFromEnv()
	b := &Bus{SkipNullCleanup: os.Getenv("WEBTV_SKIP_NULL_CLEANUP") == "1", TraceNullFlow: os.Getenv("WEBTV_TRACE_NULL_FLOW") == "1", TraceDeviceField: os.Getenv("WEBTV_TRACE_DEVICE_FIELD") == "1", TracePage: os.Getenv("WEBTV_TRACE_PAGE") == "1", TraceWebTVPort: os.Getenv("WEBTV_TRACE_WTV_PORT") == "1", TraceRIOControl: os.Getenv("WEBTV_TRACE_RIO_CONTROL") == "1", TraceVideoASIC: os.Getenv("WEBTV_TRACE_VIDEO") == "1", TraceVideoFrame: os.Getenv("WEBTV_TRACE_VIDEO_FRAME") == "1", TraceModemTX: os.Getenv("WEBTV_TRACE_MODEM_TX") == "1", TraceModemPPP: os.Getenv("WEBTV_TRACE_MODEM_PPP") == "1", TraceDeviceObject: os.Getenv("WEBTV_TRACE_DEVICE_OBJECT") == "1", TraceDevicePage: os.Getenv("WEBTV_TRACE_DEVICE_PAGE") == "1", TracePacketQueue: os.Getenv("WEBTV_TRACE_PACKET_QUEUE") == "1", SyntheticPPP: os.Getenv("WEBTV_SYNTHETIC_PPP") == "1", CompatShims: os.Getenv("WEBTV_COMPAT_SHIMS") == "1", ForceModemDeviceStatus: forceModemDeviceStatus, PersistModemDeviceStatus: os.Getenv("WEBTV_PERSIST_MODEM_STATUS") == "1", ModemDeviceStatus: modemDeviceStatus, PersistModemDeviceFields: persistModemDeviceFields, ModemDeviceField18C: modemDeviceField18C, ModemDeviceField190: modemDeviceField190, ProbeModemQueue: probeModemQueue, ModemQueueProbePtr: modemQueueProbePtr, ModemQueueProbeMask: modemQueueProbeMask, ModemNoCarrier: os.Getenv("WEBTV_MODEM_NO_CARRIER") == "1", ModemFirmware: make(map[uint32]byte)}
	b.Asic = NewSoloASIC()
	b.IRInput = make(chan byte, 16)
	b.Asic.VideoMemoryRead = func(addr uint32, dst []byte) bool {
		if len(dst) == 0 || uint64(addr)+uint64(len(dst)) > uint64(len(b.RAM)) {
			return false
		}
		copy(dst, b.RAM[addr:addr+uint32(len(dst))])
		return true
	}
	b.IDE = NewIDEController(b.Asic)
	b.PollCnt = make(map[uint32]uint64)
	return b
}

func modemDeviceStatusFromEnv() (uint32, bool) {
	if value := strings.TrimPrefix(strings.TrimPrefix(os.Getenv("WEBTV_MODEM_STATUS"), "0x"), "0X"); value != "" {
		parsed, err := strconv.ParseUint(value, 16, 32)
		if err == nil {
			return uint32(parsed), true
		}
	}
	if os.Getenv("WEBTV_FORCE_MODEM_STATUS") == "1" {
		return 0x76610000, true
	}
	return 0, false
}

func modemDeviceFieldsFromEnv() (uint32, uint32, bool) {
	parts := strings.Split(os.Getenv("WEBTV_MODEM_DEVICE_FIELDS"), ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	parse := func(value string) (uint32, bool) {
		value = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "0x"), "0X")
		parsed, err := strconv.ParseUint(value, 16, 32)
		return uint32(parsed), err == nil
	}
	field18C, ok18C := parse(parts[0])
	field190, ok190 := parse(parts[1])
	return field18C, field190, ok18C && ok190
}

func modemQueueProbeFromEnv() (uint32, uint32, bool) {
	parts := strings.Split(os.Getenv("WEBTV_MODEM_QUEUE_PROBE"), ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	parse := func(value string) (uint32, bool) {
		value = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "0x"), "0X")
		parsed, err := strconv.ParseUint(value, 16, 32)
		return uint32(parsed), err == nil
	}
	ptr, okPtr := parse(parts[0])
	mask, okMask := parse(parts[1])
	return ptr, mask, okPtr && okMask && ptr != 0 && mask != 0
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
const webTVPortSecondaryProbeAddress uint32 = 0x057FFFF3
const webTVPortSecondaryProbeWordAddress uint32 = 0x057FFFF0
const webTVPortBase uint32 = 0x04F00000
const webTVPortSize uint32 = 0x18
const modemCharCycles uint64 = 14500

func (b *Bus) traceWebTVPort(op string, p, width, value uint32) {
	if !b.TraceWebTVPort || b.WebTVPortTraceCount >= 128 || !((p >= webTVPortBase && p < webTVPortBase+webTVPortSize && width <= webTVPortBase+webTVPortSize-p) || (p >= webTVPortProbeWordAddress && p < webTVPortProbeWordAddress+4 && width <= webTVPortProbeWordAddress+4-p) || (p >= webTVPortSecondaryProbeWordAddress && p < webTVPortSecondaryProbeWordAddress+4 && width <= webTVPortSecondaryProbeWordAddress+4-p)) || b.CPU == nil {
		return
	}
	b.WebTVPortTraceCount++
	fmt.Printf("WebTV port %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X width=%d val=0x%08X v0=0x%08X a0=0x%08X a1=0x%08X ra=0x%08X\n", op, b.WebTVPortTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), p, width, value, uint32(b.CPU.Regs[2]), uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[31]))
}

func webTVPortResponse(p, width uint32) (uint32, bool) {
	switch os.Getenv("WEBTV_WTV_PORT_RESPONSE") {
	case "", "none":
		return 0, false
	case "type2":
		if p == webTVPortProbeAddress && width == 1 {
			return 2, true
		}
		if p == webTVPortProbeWordAddress && width == 4 {
			return 0xFFFFFFFF, true
		}
	case "type2-zero":
		if p == webTVPortProbeAddress && width == 1 {
			return 2, true
		}
		if p == webTVPortProbeWordAddress && width == 4 {
			return 0, true
		}
	case "type2-word2":
		if p == webTVPortProbeAddress && width == 1 {
			return 2, true
		}
		if p == webTVPortProbeWordAddress && width == 4 {
			return 2, true
		}
	case "type0", "type1", "type3", "type4":
		if p == webTVPortProbeAddress && width == 1 {
			switch os.Getenv("WEBTV_WTV_PORT_RESPONSE") {
			case "type0":
				return 0, true
			case "type1":
				return 1, true
			case "type3":
				return 3, true
			default:
				return 4, true
			}
		}
		if p == webTVPortProbeWordAddress && width == 4 {
			return 0xFFFFFFFF, true
		}
	case "secondary-type2":
		if p == webTVPortSecondaryProbeAddress && width == 1 {
			return 2, true
		}
		if p == webTVPortSecondaryProbeWordAddress && width == 4 {
			return 0xFFFFFFFF, true
		}
	}
	return 0, false
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

func (b *Bus) traceVideoASICWrite(off, width, value uint32) {
	if !b.TraceVideoASIC || b.VideoASICTraceCount >= 256 || b.CPU == nil || !((off < 0x1A0 && off+width > 0x80) || (off >= 0x3000 && off < 0x3020) || (off >= 0x6000 && off < 0x60A0) || (off >= 0x9080 && off < 0x90B0)) {
		return
	}
	b.VideoASICTraceCount++
	fmt.Printf("video ASIC write #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X off=0x%04X width=%d val=0x%08X VID_NSTART=0x%08X VIDStat=0x%08X VIDEn=0x%08X BusStat=0x%08X BusEn=0x%08X\n", b.VideoASICTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), off, width, value, b.Asic.VIDNStart, b.Asic.VIDIntStat, b.Asic.VIDIntEnable, b.Asic.BusIntStat, b.Asic.BusIntEnable)
}

func (b *Bus) traceVideoFrameWrite(p, width, value uint32) {
	base := b.Asic.VIDNStart & GLOBAL_MASK
	if base == 0 && b.TraceVideoFrame {
		base = 0x0000B740
	}
	end := base + uint32(videoWidth*videoHeight*2)
	if !b.TraceVideoFrame || b.VideoFrameWriteCount >= 256 || b.CPU == nil || base == 0 || p >= end || p+width <= base || value == 0xC0EDBABE || value == 0x10801080 || value == 0x2383237F || value == 0x3186317F || b.CPU.PC >= 0x8025CC40 && b.CPU.PC < 0x8025CD00 {
		return
	}
	b.VideoFrameWriteCount++
	fmt.Printf("video frame RAM write #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X width=%d val=0x%08X VID_NSTART=0x%08X\n", b.VideoFrameWriteCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), p, width, value, b.Asic.VIDNStart)
}

func (b *Bus) traceVideoTableRead(p, width, value uint32) {
	if !b.TraceVideoASIC || b.VideoTableReadCount >= 128 || b.CPU == nil || p < 0x32FB00 || p >= 0x32FB30 {
		return
	}
	b.VideoTableReadCount++
	fmt.Printf("video dispatch table read #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X width=%d val=0x%08X a0=0x%08X a1=0x%08X a2=0x%08X a3=0x%08X ra=0x%08X\n", b.VideoTableReadCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), 0x80000000|p, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[6]), uint32(b.CPU.Regs[7]), uint32(b.CPU.Regs[31]))
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

func (b *Bus) traceDevicePage(op string, rp, width, value uint32) {
	if !b.TraceDevicePage || b.DevicePageTraceCount >= 16384 || rp >= 0x590 || rp+width <= 0x400 || b.CPU == nil {
		return
	}
	b.DevicePageTraceCount++
	fmt.Printf("device page %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X phys=0x%08X width=%d val=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", op, b.DevicePageTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), rp, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[17]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) traceModemGlobal(op string, rp, width, value uint32) {
	if !b.TraceDeviceObject || b.ModemGlobalTraceCount >= 128 || rp >= 0x1CFC || rp+width <= 0x1CF8 || b.CPU == nil {
		return
	}
	b.ModemGlobalTraceCount++
	fmt.Printf("modem global %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X phys=0x%08X width=%d val=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", op, b.ModemGlobalTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), rp, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[17]), uint32(b.CPU.Regs[31]))
}

func (b *Bus) tracePacketQueue(op string, rp, width, value uint32) {
	global := rp < 0x1A4C && rp+width > 0x1A3C
	nodeBase := uint32(0)
	if b.ProbeModemQueue && b.ModemQueueProbePtr != 0 {
		nodeBase = b.phys(b.ModemQueueProbePtr)
	}
	node := b.TracePacketQueue && b.ProbeModemQueue && nodeBase != 0 && rp >= nodeBase && rp+width <= nodeBase+0x80
	if !b.TracePacketQueue || b.PacketQueueTraceCount >= 512 || (!global && !node) || b.CPU == nil {
		return
	}
	b.PacketQueueTraceCount++
	name := "packet queue"
	if node {
		name = "packet queue node"
	}
	fmt.Printf("%s %s #%d step=%d PC=0x%08X execPC=0x%08X inst=0x%08X phys=0x%08X width=%d val=0x%08X a0=0x%08X a1=0x%08X s0=0x%08X s1=0x%08X ra=0x%08X\n", name, op, b.PacketQueueTraceCount, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), rp, width, value, uint32(b.CPU.Regs[4]), uint32(b.CPU.Regs[5]), uint32(b.CPU.Regs[16]), uint32(b.CPU.Regs[17]), uint32(b.CPU.Regs[31]))
}

func modemSRAMOffset(p, width uint32) (uint32, bool) {
	const base = uint32(0x1E000000)
	if p < base {
		return 0, false
	}
	offset := p - base
	if width == 0 || offset >= modemSRAMSize || width > modemSRAMSize-offset {
		return 0, false
	}
	return offset, true
}

func (b *Bus) Read8(addr uint32) uint8 {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 1); ok {
		v := b.RAM[rp]
		b.traceVideoTableRead(rp, 1, uint32(v))
		b.traceDevicePage("read", rp, 1, uint32(v))
		b.traceModemGlobal("read", rp, 1, uint32(v))
		b.tracePacketQueue("read", rp, 1, uint32(v))
		return v
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX) {
		b.tracePageRead(p, 1)
		off := p - ROM_BASE
		return b.ROM[off]
	}
	if b.ModemBackdoor {
		if off, ok := modemSRAMOffset(p, 1); ok {
			return b.ModemSRAM[off]
		}
	} else if p >= 0x1E000000 && p < 0x1E000020 {
		return b.modemRead8(p - 0x1E000000)
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		v := b.Asic.Read(off)
		b.traceRIOControl("read", off, 1, v&0xFF)
		return uint8(v)
	}
	if v, ok := webTVPortResponse(p, 1); ok {
		b.traceWebTVPort("read", p, 1, v)
		return uint8(v)
	}
	b.traceWebTVPort("read", p, 1, 0xFF)
	return 0xFF
}

func (b *Bus) Read16(addr uint32) uint16 {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 2); ok {
		v := binary.BigEndian.Uint16(b.RAM[rp:])
		b.traceVideoTableRead(rp, 2, uint32(v))
		b.traceDevicePage("read", rp, 2, uint32(v))
		b.traceModemGlobal("read", rp, 2, uint32(v))
		b.tracePacketQueue("read", rp, 2, uint32(v))
		return v
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX)-1 {
		b.tracePageRead(p, 2)
		off := p - ROM_BASE
		return binary.BigEndian.Uint16(b.ROM[off:])
	}
	if b.ModemBackdoor {
		if off, ok := modemSRAMOffset(p, 2); ok {
			return uint16(b.ModemSRAM[off])
		}
	} else if p >= 0x1E000000 && p < 0x1E000020 {
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
	if v, ok := webTVPortResponse(p, 2); ok {
		b.traceWebTVPort("read", p, 2, v)
		return uint16(v)
	}
	b.traceWebTVPort("read", p, 2, 0xFFFF)
	return 0xFFFF
}

func (b *Bus) Read32(addr uint32) uint32 {
	p := b.phys(addr)
	if b.PersistModemDeviceStatus {
		switch p {
		case 0x588:
			b.traceDeviceObjectRead(p, b.ModemDeviceStatus)
			return b.ModemDeviceStatus
		case 0x58C, 0x590:
			if b.PersistModemDeviceFields {
				value := b.ModemDeviceField18C
				if p == 0x590 {
					value = b.ModemDeviceField190
				}
				b.traceDeviceObjectRead(p, value)
				return value
			}
		}
	}
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
	if b.ModemBackdoor {
		if off, ok := modemSRAMOffset(p, 4); ok {
			return uint32(b.ModemSRAM[off])
		}
	} else if p >= 0x1E000000 && p < 0x1E000020 {
		return uint32(b.modemRead8(p - 0x1E000000))
	}
	if rp, ok := ramMirror(p, 4); ok {
		v := binary.BigEndian.Uint32(b.RAM[rp:])
		b.traceVideoTableRead(rp, 4, v)
		b.traceDevicePage("read", rp, 4, v)
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
	if v, ok := webTVPortResponse(p, 4); ok {
		b.traceWebTVPort("read", p, 4, v)
		return v
	}
	b.traceWebTVPort("read", p, 4, 0xFFFFFFFF)
	return 0xFFFFFFFF
}

func (b *Bus) tracePageRead(p, width uint32) {
	if !b.TracePage || b.CPU == nil || b.TracePageCount >= 512 {
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
		b.traceVideoFrameWrite(rp, 1, val&0xFF)
		b.traceDevicePage("write", rp, 1, val&0xFF)
		b.traceDeviceObjectWrite(rp, 1, val&0xFF)
		b.traceModemGlobal("write", rp, 1, val&0xFF)
		b.tracePacketQueue("write", rp, 1, val&0xFF)
		if b.Tracking && ((rp >= 0x330 && rp <= 0x590) || (rp >= 0x288 && rp <= 0x28B) || (rp >= 0x101C && rp <= 0x101F) || (rp >= 0x16A0 && rp <= 0x16A3) || (rp >= 0x1710 && rp <= 0x171B) || (rp >= 0x17A0 && rp <= 0x17A3) || (rp >= 0x1904 && rp <= 0x1909) || (rp >= 0x1930 && rp <= 0x1933) || (rp >= 0x1CF0 && rp <= 0x1CF7) || (rp >= 0x2240 && rp <= 0x224E) || (rp >= 0x2730 && rp <= 0x2733) || (rp >= 0x6340 && rp <= 0x6343) || (rp >= 0x6754 && rp <= 0x6757) || (rp >= 0x7E930 && rp <= 0x7E960) || (rp >= 0x80A88 && rp <= 0x80A98) || (rp >= 0xA510 && rp <= 0xA690) || rp == 0x1E74) && b.CPU != nil {
			fmt.Printf("RAM watch8 PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X phys=0x%08X val=0x%02X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, rp, val&0xFF)
		}
		b.RAM[rp] = byte(val)
		return
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		b.traceRIOControl("write", off, 1, val&0xFF)
		b.traceVideoASICWrite(off, 1, val&0xFF)
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
	if b.ModemBackdoor {
		if off, ok := modemSRAMOffset(p, 1); ok {
			b.ModemSRAM[off] = byte(val)
			return
		}
	} else if p >= 0x1E000000 && p < 0x1E000020 {
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
		b.traceVideoFrameWrite(rp, 2, val&0xFFFF)
		b.traceDevicePage("write", rp, 2, val&0xFFFF)
		b.traceDeviceObjectWrite(rp, 2, val&0xFFFF)
		b.traceModemGlobal("write", rp, 2, val&0xFFFF)
		b.tracePacketQueue("write", rp, 2, val&0xFFFF)
		if b.Tracking && ((rp <= 0x590 && rp+1 >= 0x330) || (rp <= 0x28B && rp+1 >= 0x288) || (rp <= 0x101F && rp+1 >= 0x101C) || (rp <= 0x16A3 && rp+1 >= 0x16A0) || (rp <= 0x171B && rp+1 >= 0x1710) || (rp <= 0x17A3 && rp+1 >= 0x17A0) || (rp <= 0x1909 && rp+1 >= 0x1904) || (rp <= 0x1933 && rp+1 >= 0x1930) || (rp <= 0x1CF7 && rp+1 >= 0x1CF0) || (rp <= 0x224E && rp+1 >= 0x2240) || (rp <= 0x2733 && rp+1 >= 0x2730) || (rp <= 0x6343 && rp+1 >= 0x6340) || (rp <= 0x6757 && rp+1 >= 0x6754) || (rp <= 0x7E960 && rp+1 >= 0x7E930) || (rp <= 0x80A98 && rp+1 >= 0x80A88) || (rp <= 0xA690 && rp+1 >= 0xA510) || (rp <= 0x1E74 && rp+1 >= 0x1E74)) && b.CPU != nil {
			fmt.Printf("RAM watch16 PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X phys=0x%08X val=0x%04X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, rp, val&0xFFFF)
		}
		binary.BigEndian.PutUint16(b.RAM[rp:], uint16(val))
		return
	}
	if p >= 0x04000000 && p < 0x04010000 {
		off := p & 0xFFFF
		b.traceRIOControl("write", off, 2, val&0xFFFF)
		b.traceVideoASICWrite(off, 2, val&0xFFFF)
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
	if b.ModemBackdoor {
		if off, ok := modemSRAMOffset(p, 2); ok {
			b.ModemSRAM[off] = byte(val)
			return
		}
	} else if p >= 0x1E000000 && p < 0x1E000020 {
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
	if b.ModemBackdoor {
		if off, ok := modemSRAMOffset(p, 4); ok {
			b.ModemSRAM[off] = byte(val)
			return
		}
	} else if p >= 0x1E000000 && p < 0x1E000020 {
		b.modemWrite(p-0x1E000000, val, 4)
		return
	}
	if rp, ok := ramMirror(p, 4); ok {
		b.traceVideoFrameWrite(rp, 4, val)
		b.traceDevicePage("write", rp, 4, val)
		b.traceDeviceObjectWrite(rp, 4, val)
		b.traceModemGlobal("write", rp, 4, val)
		b.tracePacketQueue("write", rp, 4, val)
		if b.Tracking && ((rp <= 0x590 && rp+3 >= 0x330) || (rp <= 0x28B && rp+3 >= 0x288) || (rp <= 0x101F && rp+3 >= 0x101C) || (rp <= 0x16A3 && rp+3 >= 0x16A0) || (rp <= 0x171B && rp+3 >= 0x1710) || (rp <= 0x17A3 && rp+3 >= 0x17A0) || (rp <= 0x1907 && rp+3 >= 0x18F0) || (rp <= 0x1933 && rp+3 >= 0x1930) || (rp <= 0x1CF7 && rp+3 >= 0x1CF0) || (rp <= 0x224E && rp+3 >= 0x2240) || (rp <= 0x2733 && rp+3 >= 0x2730) || (rp <= 0x6343 && rp+3 >= 0x6340) || (rp <= 0x6757 && rp+3 >= 0x6754) || (rp <= 0x7E960 && rp+3 >= 0x7E930) || (rp <= 0x80A98 && rp+3 >= 0x80A88) || (rp <= 0xA690 && rp+3 >= 0xA510) || (rp <= 0x1E74 && rp+3 >= 0x1E74) || (rp <= 0xEEDD0 && rp+3 >= 0xEEDA8) || (rp <= 0xF0B93 && rp+3 >= 0xF0B70)) && b.CPU != nil {
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
		b.traceVideoASICWrite(off, 4, val)
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
			if b.ForceModemDeviceStatus {
				binary.BigEndian.PutUint32(b.RAM[0x588:], b.ModemDeviceStatus)
				fmt.Printf("forced modem device status step=%d phys=0x588 val=0x%08X\n", b.CPU.Steps, b.ModemDeviceStatus)
			}
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
				b.ModemFirmware = make(map[uint32]byte)
				b.ModemFirmwareRecords = 0
				b.ModemFirmwareBytes = 0
				b.ModemFirmwareReady = false
				b.queueModemRX([]byte("\n\nDownload Modem Firmware .."))
				b.ModemStage = 1
				if b.CPU != nil {
					fmt.Printf("modem stage 1 queued step=%d writes=%d line=%q\n", b.CPU.Steps, b.ModemWriteCnt, line)
				}
			}
			if b.ModemStage >= 1 && b.ModemStage < 3 && strings.HasPrefix(line, "S") {
				if record, err := parseModemSRecord([]byte(line)); err == nil {
					switch record.kind {
					case '3':
						for i, value := range record.data {
							b.ModemFirmware[record.address+uint32(i)] = value
							if record.address+uint32(i) < modemSRAMSize {
								b.ModemSRAM[record.address+uint32(i)] = value
							}
						}
						b.ModemFirmwareRecords++
						b.ModemFirmwareBytes += len(record.data)
					case '7':
						if b.ModemFirmwareRecords != 0 && b.ModemFirmwareBytes != 0 {
							b.ModemFirmwareReady = true
							b.queueModemRX([]byte("."))
							b.ModemStage = 3
							if b.CPU != nil {
								fmt.Printf("modem stage 2 queued step=%d writes=%d line=%q records=%d bytes=%d\n", b.CPU.Steps, b.ModemWriteCnt, line, b.ModemFirmwareRecords, b.ModemFirmwareBytes)
							}
						}
					}
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
			if b.ModemNoCarrier {
				if !b.ModemNoCarrierQueued && b.ModemNoCarrierAt == 0 {
					b.ModemNoCarrierAt = b.Asic.Cycles + modemNoCarrierDelayCycles
				}
			} else if current == 0 && b.ModemReadyAt == 0 {
				b.ModemReadyAt = b.Asic.Cycles + 10000
			}
		} else {
			current = 0
			b.ModemReadyAt = 0
			b.ModemNoCarrierAt = 0
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
	if b.Asic.IRStatCntl&1 != 0 && b.Asic.IRSample != 0 {
		select {
		case command := <-b.IRInput:
			b.Asic.QueueIRTransitions(remoteIRTransitions(command))
		default:
		}
	}
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
	if b.ModemNoCarrierAt != 0 && b.Asic.Cycles >= b.ModemNoCarrierAt {
		b.ModemNoCarrierAt = 0
		b.ModemNoCarrierQueued = true
		b.queueModemWireRX([]byte("\r\nNO CARRIER\r\n"))
		if b.CPU != nil {
			fmt.Printf("modem no-carrier queued step=%d\n", b.CPU.Steps)
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
	for _, packet := range b.ModemPPPDecoder.Push(data) {
		b.ModemPPPFrames++
		b.ModemLastPPPProtocol = packet.Protocol
		b.ModemLastPPPInfo = append(b.ModemLastPPPInfo[:0], packet.Information...)
		b.seedModemQueueProbe()
		if b.TraceModemPPP && b.CPU != nil {
			fmt.Printf("modem PPP RX frame #%d step=%d protocol=0x%04X infoBytes=%d\n", b.ModemPPPFrames, b.CPU.Steps, packet.Protocol, len(packet.Information))
		}
	}
	b.ModemWireRX = append(b.ModemWireRX, data...)
	if b.ModemNextRXAt == 0 || b.ModemNextRXAt <= b.Asic.Cycles {
		b.ModemNextRXAt = b.Asic.Cycles + modemCharCycles
	}
	b.releaseModemRX()
}

func (b *Bus) seedModemQueueProbe() {
	if !b.ProbeModemQueue || b.ModemQueueProbeSeeded {
		return
	}
	b.ModemQueueProbeSeeded = true
	ptr := b.ModemQueueProbePtr
	mask := b.ModemQueueProbeMask
	for offset := uint32(0); offset < 0x40; offset += 4 {
		b.Write32(ptr+offset, 0)
	}
	b.Write32(ptr+0x04, ptr+0x40)
	b.Write32(ptr+0x0C, 0x00000004)
	b.Write32(ptr+0x18, mask)
	b.Write32(0x80001A3C, ptr)
	b.Write32(0x80001A40, mask)
	fmt.Printf("modem queue probe seeded ptr=0x%08X mask=0x%08X\n", ptr, mask)
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
