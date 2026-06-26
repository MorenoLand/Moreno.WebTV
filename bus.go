package main

import (
	"encoding/binary"
	"fmt"
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
	RAM           [RAM_SIZE]byte
	ROM           [ROM_MAX]byte
	ROMSz         uint32
	Asic          *SoloASIC
	IDE           *IDEController
	CPU           *CPU
	AsicReadCnt   [0x10000]uint64
	Tracking      bool
	PollCnt       map[uint32]uint64
	RIOReadCnt    uint64
	RIOWriteCnt   uint64
	ModemReadCnt  uint64
	ModemWriteCnt uint64
	ModemRX       []byte
	ModemTXLine   []byte
	ModemStage    int
	ModemStageAt  uint64
}

func NewBus() *Bus {
	b := &Bus{}
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
	if p < RAM_APERTURE && p+width <= RAM_APERTURE {
		return p % RAM_SIZE, true
	}
	return 0, false
}

func (b *Bus) Read8(addr uint32) uint8 {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 1); ok {
		return b.RAM[rp]
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX) {
		off := p - ROM_BASE
		return b.ROM[off]
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		return b.modemRead8(p - 0x1E000000)
	}
	if p >= 0x04000000 && p < 0x05000000 {
		return uint8(b.Asic.Read(p & 0xFFFF))
	}
	return 0xFF
}

func (b *Bus) Read16(addr uint32) uint16 {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 2); ok {
		return binary.BigEndian.Uint16(b.RAM[rp:])
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX)-1 {
		off := p - ROM_BASE
		return binary.BigEndian.Uint16(b.ROM[off:])
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		return uint16(b.modemRead8(p - 0x1E000000))
	}
	if p >= 0x04000000 && p < 0x05000000 {
		return uint16(b.Asic.Read(p & 0xFFFF))
	}
	if p >= 0x1E400000 && p <= 0x1E80001F {
		return uint16(b.IDE.Read32(p - 0x1E400000))
	}
	return 0xFFFF
}

func (b *Bus) Read32(addr uint32) uint32 {
	p := b.phys(addr)
	if p >= 0x04000000 && p < 0x05000000 {
		if b.Tracking {
			b.AsicReadCnt[p&0xFFFF]++
		}
		off := p & 0xFFFF
		v := b.Asic.Read(off)
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
		if b.Tracking && ((rp >= 0x710000 && rp <= 0x71FFFF) || (rp >= 0x7F0000 && rp <= 0x7FFFFF)) {
			b.PollCnt[rp]++
		}
		return binary.BigEndian.Uint32(b.RAM[rp:])
	}
	if p >= ROM_BASE && p < ROM_BASE+uint32(ROM_MAX)-3 {
		off := p - ROM_BASE
		return binary.BigEndian.Uint32(b.ROM[off:])
	}
	return 0xFFFFFFFF
}

func (b *Bus) Write8(addr, val uint32) {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 1); ok {
		if ((rp >= 0x330 && rp <= 0x337) || (rp >= 0x288 && rp <= 0x28B) || (rp >= 0x101C && rp <= 0x101F) || (rp >= 0x1904 && rp <= 0x1909) || (rp >= 0x1CF0 && rp <= 0x1CF7) || (rp >= 0x2240 && rp <= 0x224E) || (rp >= 0x2730 && rp <= 0x2733) || rp == 0x1E74) && b.CPU != nil {
			fmt.Printf("RAM watch8 PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X phys=0x%08X val=0x%02X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, rp, val&0xFF)
		}
		b.RAM[rp] = byte(val)
		return
	}
	if p >= 0x04000000 && p < 0x05000000 {
		if (p&0xFFFF) == 0x90A8 && b.CPU != nil {
			b.RIOWriteCnt++
			if b.RIOWriteCnt <= 64 || b.RIOWriteCnt%1000 == 0 {
				fmt.Printf("RIO write8 #%d step=%d PC=0x%08X execPC=0x%08X off=0x%04X val=0x%02X cycles=0x%016X ptr=0x%08X\n",
					b.RIOWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, p&0xFFFF, val&0xFF, b.Asic.Cycles, binary.BigEndian.Uint32(b.RAM[0x1C90:]))
			}
		}
		b.Asic.Write(p&0xFFFF, val)
		return
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		b.modemWrite(p-0x1E000000, val, 1)
		return
	}
}

func (b *Bus) Write16(addr, val uint32) {
	p := b.phys(addr)
	if rp, ok := ramMirror(p, 2); ok {
		if ((rp <= 0x337 && rp+1 >= 0x330) || (rp <= 0x28B && rp+1 >= 0x288) || (rp <= 0x101F && rp+1 >= 0x101C) || (rp <= 0x1909 && rp+1 >= 0x1904) || (rp <= 0x1CF7 && rp+1 >= 0x1CF0) || (rp <= 0x224E && rp+1 >= 0x2240) || (rp <= 0x2733 && rp+1 >= 0x2730) || (rp <= 0x1E74 && rp+1 >= 0x1E74)) && b.CPU != nil {
			fmt.Printf("RAM watch16 PC=0x%08X execPC=0x%08X inst=0x%08X addr=0x%08X phys=0x%08X val=0x%04X\n",
				b.CPU.PC, b.CPU.excPC, b.Read32(b.CPU.excPC), addr, rp, val&0xFFFF)
		}
		binary.BigEndian.PutUint16(b.RAM[rp:], uint16(val))
		return
	}
	if p >= 0x04000000 && p < 0x05000000 {
		if (p&0xFFFF) == 0x90A8 && b.CPU != nil {
			b.RIOWriteCnt++
			if b.RIOWriteCnt <= 64 || b.RIOWriteCnt%1000 == 0 {
				fmt.Printf("RIO write16 #%d step=%d PC=0x%08X execPC=0x%08X off=0x%04X val=0x%04X cycles=0x%016X ptr=0x%08X\n",
					b.RIOWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, p&0xFFFF, val&0xFFFF, b.Asic.Cycles, binary.BigEndian.Uint32(b.RAM[0x1C90:]))
			}
		}
		b.Asic.Write(p&0xFFFF, val)
		return
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		b.modemWrite(p-0x1E000000, val, 2)
		return
	}
}

func (b *Bus) Write32(addr, val uint32) {
	p := b.phys(addr)
	if p >= 0x1E400000 && p <= 0x1E80001F {
		b.IDE.Write32(p-0x1E400000, val)
		return
	}
	if p >= 0x1E000000 && p < 0x1E000020 {
		b.modemWrite(p-0x1E000000, val, 4)
		return
	}
	if rp, ok := ramMirror(p, 4); ok {
		if ((rp <= 0x337 && rp+3 >= 0x330) || (rp <= 0x28B && rp+3 >= 0x288) || (rp <= 0x101F && rp+3 >= 0x101C) || (rp <= 0x1907 && rp+3 >= 0x18F0) || (rp <= 0x1CF7 && rp+3 >= 0x1CF0) || (rp <= 0x224E && rp+3 >= 0x2240) || (rp <= 0x2733 && rp+3 >= 0x2730) || (rp <= 0x1E74 && rp+3 >= 0x1E74) || (rp <= 0xEEDD0 && rp+3 >= 0xEEDA8) || (rp <= 0xF0B93 && rp+3 >= 0xF0B70)) && b.CPU != nil {
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
	if p >= 0x04000000 && p < 0x05000000 {
		if (p&0xFFFF) == 0x90A8 && b.CPU != nil {
			b.RIOWriteCnt++
			if b.RIOWriteCnt <= 64 || b.RIOWriteCnt%1000 == 0 {
				fmt.Printf("RIO write32 #%d step=%d PC=0x%08X execPC=0x%08X off=0x%04X val=0x%08X cycles=0x%016X ptr=0x%08X\n",
					b.RIOWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, p&0xFFFF, val, b.Asic.Cycles, binary.BigEndian.Uint32(b.RAM[0x1C90:]))
			}
		}
		b.Asic.Write(p&0xFFFF, val)
		return
	}
}

func (b *Bus) modemRead8(off uint32) uint8 {
	b.ModemReadCnt++
	var v uint8
	switch off {
	case 0x00:
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
	case 0x14:
		if len(b.ModemRX) != 0 {
			v = 0x21
			break
		}
		v = 0x20
	case 0x18:
		if len(b.ModemRX) != 0 {
			v = 0x80
		}
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
	if b.CPU != nil && (b.ModemWriteCnt <= 128 || b.ModemWriteCnt%1000000 == 0) {
		fmt.Printf("modem write #%d step=%d PC=0x%08X execPC=0x%08X off=0x%02X width=%d val=0x%X\n", b.ModemWriteCnt, b.CPU.Steps, b.CPU.PC, b.CPU.excPC, off, width, val)
	}
	if off == 0 {
		ch := byte(val)
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
	}
}

func (b *Bus) queueModemRX(data []byte) {
	b.ModemRX = append(b.ModemRX, data...)
	b.Asic.DevIntStat |= 1
	b.Asic.BusIntStat |= BUS_INT_DEVICE
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
