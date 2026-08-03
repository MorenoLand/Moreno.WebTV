package main

import (
	"fmt"
	"os"
)

type SoloASIC struct {
	ReadHits  map[uint32]uint64
	Regs      map[uint32]uint32
	WriteLog  map[uint32]uint32
	ChipID    uint32
	SysConfig uint32
	CHPCntl   uint32
	WDEnable  bool
	WDValue   uint32

	BusIntEnable uint32
	BusIntStat   uint32

	TimerCompare uint32
	TimIntEnable uint32
	TimIntStat   uint32

	DevIntEnable  uint32
	DevIntStat    uint32
	RIOIntEnable  uint32
	RIOIntStat    uint32
	VIDIntEnable  uint32
	VIDIntStat    uint32
	VIDNStart     uint32
	AUDIntEnable  uint32
	AUDIntStat    uint32
	GPIOIntEnable uint32
	GPIOIntStat   uint32

	ErrEnable uint8
	ErrStat   uint8

	MemCntl   uint32
	MemRefCnt uint32
	MemData   uint32
	MemCmd    uint32
	MemTiming uint32

	ROMCntl0 uint32
	ROMCntl1 uint32

	LEDState uint32
	IRCntl   uint32
	NVCntl   uint8

	GPIOIn      uint32
	GPIOOut     uint32
	GPIOInMask  uint32
	GPIOOutMask uint32

	PPortEnabled   uint32
	PPortData      uint32
	PPortCtrl      uint32
	PPortStatus    uint32
	PPortIntEnable uint32
	PPortIntStat   uint32

	BootMode    uint32
	UseBootMode uint32
	ResetCause  uint32

	Fence1Addr uint32
	Fence1Mask uint32
	Fence2Addr uint32
	Fence2Mask uint32

	J1FenLAddr uint32
	J1FenHAddr uint32
	J2FenLAddr uint32
	J2FenHAddr uint32

	Cycles         uint64
	RIOPollReadyAt uint64
	SysTimerNext   uint64
	VideoNext      uint64

	I2C I2CBus
}

type I2CBus struct {
	Data      [128]byte
	SDA       uint8
	SCL       uint8
	State     int
	Bits      int
	Byte      uint8
	Addr      uint8
	WriteAddr uint8
	Reading   bool
	TxCount   int
}

const (
	i2cIdle = iota
	i2cAddr
	i2cWordAddr
	i2cReadData
	i2cWriteData
)

const rioPollPeriodCycles = 200000
const sysTimerPeriodCycles = 2783333
const sysTimerStartCycles = 0xFFFFFFFFFFFFFFFF
const videoFramePeriodCycles = 2783333

func NewSoloASIC() *SoloASIC {
	a := &SoloASIC{
		ChipID:      0x03120000,
		SysConfig:   0x003f8700,
		VIDNStart:   0x80000000,
		MemTiming:   0xADBADFFA,
		MemCntl:     0x03,
		MemRefCnt:   0x0400,
		PPortStatus: 0x08,
		LEDState:    0xFFFFFFFF,
		ResetCause:  2, // RESETCAUSE_WATCHDOG per MAME device_reset()
		ReadHits:    make(map[uint32]uint64),
		Regs:        make(map[uint32]uint32),
	}
	a.I2C.SDA = 1
	return a
}

func (a *SoloASIC) LoadNVRAM(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	copy(a.I2C.Data[:], data)
	return nil
}

func (a *SoloASIC) Read(off uint32) uint32 {
	a.ReadHits[off]++
	switch off {
	case 0x0000:
		return a.ChipID
	case 0x0004:
		return a.CHPCntl
	case 0x0008, 0x0108:
		return a.BusIntStat
	case 0x0050:
		return a.BusIntStat
	case 0x000C, 0x010C:
		return a.BusIntEnable
	case 0x0010, 0x0110:
		return uint32(a.ErrStat)
	case 0x0014:
		return uint32(a.ErrEnable)
	case 0x0114:
		return 0
	case 0x0018:
		return 0
	case 0x001C:
		return a.Fence1Addr
	case 0x0020:
		return a.Fence1Mask
	case 0x0024:
		return a.Fence2Addr
	case 0x0028:
		return a.Fence2Mask
	case 0x0030:
		return a.WDValue
	case 0x0048:
		return uint32(a.Cycles >> 1)
	case 0x004C:
		return a.TimerCompare
	case 0x0058, 0x0060:
		return a.GPIOIntStat
	case 0x005C, 0x015C:
		return a.GPIOIntEnable
	case 0x0064:
		return 0
	case 0x0068, 0x006C, 0x0168:
		return a.AUDIntStat
	case 0x0070, 0x0170:
		return a.AUDIntEnable
	case 0x0074, 0x0078, 0x0174:
		return a.DevIntStat
	case 0x007C, 0x017C:
		return a.DevIntEnable
	case 0x0080, 0x0084, 0x0180:
		return a.VIDIntStat
	case 0x0088, 0x0188:
		return a.VIDIntEnable
	case 0x008C, 0x0090, 0x018C:
		return a.RIOIntStat
	case 0x0094, 0x0098, 0x0194, 0x0198:
		return a.RIOIntEnable
	case 0x009C, 0x00A0, 0x019C:
		return a.TimIntStat
	case 0x90A0:
		if a.Cycles > 45_000_000 {
			return 0x3A
		}
		if a.Cycles >= a.RIOPollReadyAt {
			return 1
		}
		return 0
	case 0x00A4, 0x01A4:
		return a.TimIntEnable
	case 0x00A8:
		return a.ResetCause
	case 0x00B0:
		return a.J1FenLAddr
	case 0x00B4:
		return a.J1FenHAddr
	case 0x00B8:
		return a.J2FenLAddr
	case 0x00BC:
		return a.J2FenHAddr
	case 0x00C8:
		return a.BootMode
	case 0x00CC:
		return a.UseBootMode
	case 0x1000:
		return a.SysConfig
	case 0x1004:
		return a.ROMCntl0
	case 0x1008:
		return a.ROMCntl1
	case 0x4000:
		return 0
	case 0x4004:
		return a.LEDState
	case 0x4008:
		return uint32(a.IRCntl)
	case 0x400C:
		return uint32(a.NVCntl&0xE) | uint32(a.I2C.SDA)
	case 0x4010:
		return (a.GPIOIn & a.GPIOInMask) | (a.GPIOOut & a.GPIOOutMask)
	case 0x4014, 0x4114:
		return a.GPIOOut & a.GPIOOutMask
	case 0x4018, 0x4118:
		return a.GPIOInMask
	case 0x4020, 0x4024, 0x4028, 0x402C:
		return 0
	case 0x4200:
		if a.PPortEnabled == 0 {
			return 0
		}
		return a.PPortData & 0xFF
	case 0x4204:
		return pportASICControl(a.PPortCtrl)
	case 0x4208:
		if a.PPortEnabled == 0 {
			return 0
		}
		return pportASICStatus(a.PPortStatus)
	case 0x420C, 0x4210, 0x4214, 0x4218, 0x421C:
		return 0
	case 0x4220:
		return a.PPortEnabled
	case 0x4224, 0x4228:
		return a.PPortIntStat
	case 0x422C:
		return a.PPortEnabled
	case 0x5000:
		return a.MemCntl
	case 0x5004:
		return a.MemRefCnt
	case 0x5008:
		return a.MemData
	case 0x500C:
		return a.MemCmd
	case 0x5010:
		return a.MemTiming
	case 0x300C:
		return a.VIDNStart
	default:
		if v, ok := a.Regs[off]; ok {
			return v
		}
		return 0
	}
}

func (a *SoloASIC) Write(off, data uint32) {
	if a.WriteLog == nil {
		a.WriteLog = make(map[uint32]uint32)
	}
	a.WriteLog[off] = data
	switch off {
	case 0x0004:
		a.CHPCntl = data
	case 0x0008:
		a.BusIntStat |= data
	case 0x0108:
		a.BusIntStat &^= data
	case 0x000C:
		a.BusIntEnable |= data
	case 0x010C:
		a.BusIntStat &^= data
		a.BusIntEnable &^= data
	case 0x0010:
		a.ErrStat |= uint8(data) & 0xFF
	case 0x0110:
		a.ErrStat &^= uint8(data) & 0xFF
	case 0x0014:
		a.ErrEnable |= uint8(data) & 0xFF
	case 0x0114:
		a.ErrEnable &^= uint8(data) & 0xFF
	case 0x001C:
		a.Fence1Addr = data
	case 0x0020:
		a.Fence1Mask = data
	case 0x0024:
		a.Fence2Addr = data
	case 0x0028:
		a.Fence2Mask = data
	case 0x0030:
		a.WDValue = data
	case 0x0118:
	case 0x0048:
	case 0x004C:
		a.TimerCompare = data
		fmt.Printf("ASIC timer compare write cycles=0x%016X count=0x%08X compare=0x%08X busEn=0x%08X timEn=0x%08X timStat=0x%08X\n",
			a.Cycles, uint32(a.Cycles>>1), a.TimerCompare, a.BusIntEnable, a.TimIntEnable, a.TimIntStat)
		if data != 0 {
			a.BusIntEnable |= 1 << 3 // BUS_INT_TIMER
		}
	case 0x005C:
		a.GPIOIntEnable |= data
	case 0x015C:
		a.GPIOIntStat &^= data
		a.GPIOIntEnable &^= data
	case 0x0060:
		a.GPIOIntStat |= data
	case 0x0158:
		a.GPIOIntStat &^= data
	case 0x0064:
	case 0x0070:
		a.AUDIntEnable |= data
	case 0x0170:
		a.AUDIntStat &^= data
		a.AUDIntEnable &^= data
	case 0x006C:
		a.AUDIntStat |= data
	case 0x0168:
		a.AUDIntStat &^= data
	case 0x007C:
		a.DevIntEnable |= data
	case 0x017C:
		a.DevIntStat &^= data
		a.DevIntEnable &^= data
	case 0x0078:
		a.DevIntStat |= data
	case 0x0174:
		a.DevIntStat &^= data
		if a.DevIntStat&a.DevIntEnable == 0 {
			a.BusIntStat &^= BUS_INT_DEVICE
		}
	case 0x0088:
		a.VIDIntEnable |= data
		if a.VIDIntEnable != 0 && a.VideoNext == 0 {
			a.VideoNext = a.Cycles + videoFramePeriodCycles
		}
		a.updateVideoInterrupt()
	case 0x0188:
		a.VIDIntStat &^= data
		a.VIDIntEnable &^= data
		if a.VIDIntEnable == 0 {
			a.VideoNext = 0
		}
		a.updateVideoInterrupt()
	case 0x0084:
		a.VIDIntStat |= data
		a.updateVideoInterrupt()
	case 0x0180:
		a.VIDIntStat &^= data
		a.updateVideoInterrupt()
	case 0x0094:
		a.RIOIntEnable = data
	case 0x0098:
		a.RIOIntEnable |= data
	case 0x0194:
		a.RIOIntEnable &^= data
	case 0x0198:
		a.RIOIntStat &^= data
		a.RIOIntEnable &^= data
	case 0x0090:
		a.RIOIntStat |= data
	case 0x018C:
		a.RIOIntStat &^= data
		if a.RIOIntStat&a.RIOIntEnable == 0 {
			a.BusIntStat &^= BUS_INT_RIO
		}
	case 0x00A4:
		a.TimIntEnable |= data
		if a.TimIntEnable != 0 {
			a.BusIntEnable |= BUS_INT_TIMER
		}
		if data&(1<<2) != 0 && a.SysTimerNext == 0 {
			a.SysTimerNext = a.Cycles + sysTimerPeriodCycles
		}
		fmt.Printf("ASIC timer enable write cycles=0x%016X count=0x%08X data=0x%08X timEn=0x%08X timStat=0x%08X busEn=0x%08X\n",
			a.Cycles, uint32(a.Cycles>>1), data, a.TimIntEnable, a.TimIntStat, a.BusIntEnable)
	case 0x01A4:
		a.TimIntStat &^= data
		a.TimIntEnable &^= data
		if data&((1<<2)|(1<<3)) != 0 {
			a.SysTimerNext = 0
			a.BusIntStat &^= 1 << 3
		}
		fmt.Printf("ASIC timer disable/ack write cycles=0x%016X count=0x%08X data=0x%08X timEn=0x%08X timStat=0x%08X busEn=0x%08X\n",
			a.Cycles, uint32(a.Cycles>>1), data, a.TimIntEnable, a.TimIntStat, a.BusIntEnable)
	case 0x00A0:
		a.TimIntStat |= data
		fmt.Printf("ASIC timer stat set write cycles=0x%016X count=0x%08X data=0x%08X timEn=0x%08X timStat=0x%08X busEn=0x%08X\n",
			a.Cycles, uint32(a.Cycles>>1), data, a.TimIntEnable, a.TimIntStat, a.BusIntEnable)
	case 0x019C:
		a.TimIntStat &^= data
		if data&((1<<2)|(1<<3)) != 0 {
			a.BusIntStat &^= 1 << 3
		}
		if data&((1<<2)|(1<<3)) != 0 && a.TimIntEnable&(1<<2) != 0 {
			a.SysTimerNext = a.Cycles + sysTimerPeriodCycles
		}
		fmt.Printf("ASIC timer stat clear write cycles=0x%016X count=0x%08X data=0x%08X timEn=0x%08X timStat=0x%08X busEn=0x%08X\n",
			a.Cycles, uint32(a.Cycles>>1), data, a.TimIntEnable, a.TimIntStat, a.BusIntEnable)
	case 0x00A8:
		a.ResetCause |= data
	case 0x00AC:
		a.ResetCause &^= data & 0xFF
	case 0x90A8:
		a.RIOPollReadyAt = a.Cycles + rioPollPeriodCycles
	case 0x00B0:
		a.J1FenLAddr = data
	case 0x00B4:
		a.J1FenHAddr = data
	case 0x00B8:
		a.J2FenLAddr = data
	case 0x00BC:
		a.J2FenHAddr = data
	case 0x00C8:
		a.BootMode = data
	case 0x00CC:
		a.UseBootMode = data
	case 0x1004:
		a.ROMCntl0 = data
	case 0x1008:
		a.ROMCntl1 = data
	case 0x4004:
		a.LEDState = data
	case 0x4008:
		a.IRCntl = data & 0xFF
	case 0x400C:
		a.NVCntl = uint8(data) & 0xE
		newSCL := uint8((data >> 3) & 1)
		var newSDA uint8
		if data&0x04 != 0 {
			newSDA = uint8((data >> 1) & 1)
		} else {
			newSDA = 1
		}
		a.i2cBitBang(a.I2C.SCL, newSCL, a.I2C.SDA, newSDA)
		a.I2C.SCL = newSCL
		a.I2C.SDA = newSDA
	case 0x4014:
		a.GPIOOut |= data & a.GPIOOutMask
	case 0x4114:
		a.GPIOOut &^= data & a.GPIOOutMask
	case 0x4018:
		a.GPIOOutMask |= data
		a.GPIOInMask = ^a.GPIOOutMask
	case 0x4118:
		a.GPIOOutMask &^= data
		a.GPIOInMask = ^a.GPIOOutMask
	case 0x4020, 0x4024, 0x402C:
	case 0x4200:
		a.PPortData = data & 0xFF
	case 0x4204:
		a.PPortCtrl = pportLPTControl(data)
	case 0x420C, 0x4210, 0x4214, 0x4218, 0x421C:
	case 0x4220:
		a.PPortIntEnable = data
	case 0x4224:
		a.PPortIntStat |= data
	case 0x4228:
		a.PPortIntStat &^= data
	case 0x422C:
		a.PPortEnabled = data & 1
	case 0x5000:
		a.MemCntl = data
	case 0x5004:
		a.MemRefCnt = data
	case 0x5008:
		a.MemData = data
	case 0x500C:
		a.MemCmd = data
	case 0x5010:
		a.MemTiming = data
	case 0x300C:
		a.VIDNStart = data
	default:
		a.Regs[off] = data
	}
}

func pportASICControl(lpt uint32) uint32 {
	return ((lpt>>5)&1)<<4 | ((lpt>>3)&1)<<3 | ((lpt>>1)&1)<<2 | ((lpt>>2)&1)<<1 | lpt&1
}

func pportASICStatus(lpt uint32) uint32 {
	return ((lpt>>6)&1)<<4 | ((lpt>>4)&1)<<3 | ((lpt>>5)&1)<<2 | ((lpt>>3)&1)<<1 | (lpt>>7)&1
}

func pportLPTControl(asic uint32) uint32 {
	return ((asic>>4)&1)<<5 | ((asic>>3)&1)<<3 | ((asic>>2)&1)<<1 | ((asic>>1)&1)<<2 | asic&1
}

func (a *SoloASIC) i2cBitBang(oldSCL, newSCL, oldSDA, newSDA uint8) {
	i := &a.I2C
	// rising edge of SCL = clock bit in
	if newSCL == 1 && oldSCL == 0 {
		if i.State == i2cIdle {
			// start condition: SDA falls while SCL high
			return
		}
		if i.State == i2cReadData {
			// master clocks out bit, SDA has data from slave
			return
		}
		// shift in bit
		i.Byte = (i.Byte << 1) | newSDA
		i.Bits++
		if i.Bits == 8 {
			i.Bits = 0
			switch i.State {
			case i2cAddr:
				addr := i.Byte >> 1
				rw := i.Byte & 1
				if addr == 0x50 { // AT24C01A = 0xA0>>1
					i.SDA = 0 // ACK
					i.Reading = rw == 1
					if !i.Reading {
						i.State = i2cWordAddr
					} else {
						i.State = i2cReadData
						i.TxCount = 0
					}
				} else {
					i.SDA = 1 // NACK
					i.State = i2cIdle
				}
			case i2cWordAddr:
				i.WriteAddr = i.Byte & 0x7F
				i.SDA = 0 // ACK
				i.State = i2cWriteData
			case i2cWriteData:
				i.Data[i.WriteAddr&0x7F] = i.Byte
				i.WriteAddr++
				i.SDA = 0 // ACK
			}
		}
	}
	// falling edge of SCL = prepare for next bit or ACK read
	if newSCL == 0 && oldSCL == 1 {
		if i.State == i2cReadData {
			i.SDA = (i.Data[(int(i.WriteAddr)+i.TxCount)&0x7F] >> (7 - i.Bits)) & 1
			i.Bits++
			if i.Bits == 8 {
				i.Bits = 0
				i.TxCount++
				i.SDA = 1 // release for ACK
			}
		}
		if i.Bits == 0 && i.State != i2cIdle && i.State != i2cReadData {
			i.SDA = 1 // release SDA after ACK
		}
	}
	// start condition: SDA falls while SCL high
	if newSDA == 0 && oldSDA == 1 && newSCL == 1 {
		i.State = i2cAddr
		i.Bits = 0
		i.Byte = 0
		i.SDA = 1
	}
	// stop condition: SDA rises while SCL high
	if newSDA == 1 && oldSDA == 0 && newSCL == 1 {
		i.State = i2cIdle
		i.Bits = 0
		i.SDA = 1
	}
}

func (a *SoloASIC) Tick(cycles uint64) {
	a.Cycles += cycles
	if a.VIDIntEnable != 0 {
		if a.VideoNext == 0 {
			a.VideoNext = a.Cycles + videoFramePeriodCycles
		}
		if a.Cycles >= a.VideoNext {
			a.VIDIntStat |= a.VIDIntEnable
			a.VideoNext = a.Cycles + videoFramePeriodCycles
			a.updateVideoInterrupt()
		}
	}
	if a.TimIntEnable&BUS_INT_TIM_SYSTIMER != 0 && a.Cycles >= sysTimerStartCycles {
		if a.SysTimerNext == 0 {
			a.SysTimerNext = a.Cycles + sysTimerPeriodCycles
		}
		if a.Cycles >= a.SysTimerNext {
			a.TimIntStat |= BUS_INT_TIM_SYSTIMER
			if a.BusIntEnable&BUS_INT_TIMER != 0 {
				a.BusIntStat |= BUS_INT_TIMER
			}
		}
	}
	if a.TimerCompare != 0 && a.Cycles>>1 >= uint64(a.TimerCompare) {
		a.TimIntStat |= BUS_INT_TIM_SYSTIMER
		if a.BusIntEnable&(1<<3) != 0 {
			a.BusIntStat |= BUS_INT_TIMER
		}
	}
}

const (
	BUS_INT_TIMER        = 1 << 3 // Timer interrupt (TCOUNT == TCOMPARE)
	BUS_INT_TIM_SYSTIMER = 1 << 2 // Timer system timer
	BUS_INT_TIM_BUSTOUT  = 1 << 2 // Bus timeout / watchdog
	BUS_INT_RIO          = 1 << 5
	BUS_INT_DEVICE       = 1 << 4
	BUS_INT_VIDEO        = 1 << 7
)

func (a *SoloASIC) PendingIP2() bool {
	if a.VIDIntStat&a.VIDIntEnable != 0 {
		return true
	}
	if a.RIOIntStat&a.RIOIntEnable != 0 {
		return true
	}
	if a.BusIntStat&a.BusIntEnable != 0 {
		return true
	}
	return false
}

func (a *SoloASIC) updateVideoInterrupt() {
	if a.VIDIntStat&a.VIDIntEnable != 0 {
		a.BusIntStat |= BUS_INT_VIDEO
	} else {
		a.BusIntStat &^= BUS_INT_VIDEO
	}
}

func (a *SoloASIC) assertBusIRQ() {
	if a.RIOIntStat != 0 {
		a.BusIntStat |= BUS_INT_RIO
	}
	if a.DevIntStat != 0 {
		a.BusIntStat |= BUS_INT_DEVICE
	}
}

func (a *SoloASIC) UpdateTimerInterrupt(cpuCount, cpuCompare uint32) {
	// SOLO timer: when BUS_TCOUNT matches TCOMPARE, fire TimIntStat bit
	tcount := uint32(a.Cycles >> 1)
	if a.TimerCompare != 0 && tcount >= a.TimerCompare {
		a.TimIntStat |= BUS_INT_TIM_SYSTIMER
	}
}

func (a *SoloASIC) PrintTopReads(n int) {
	type kv struct {
		Off  uint32
		Hits uint64
	}
	var sorted []kv
	for off, hits := range a.ReadHits {
		sorted = append(sorted, kv{off, hits})
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
	fmt.Printf("Top %d ASIC polled registers:\n", n)
	for i := 0; i < n; i++ {
		fmt.Printf("  0x%04X: %d reads\n", sorted[i].Off, sorted[i].Hits)
	}
}
