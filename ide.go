package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	ATA_STAT_BSY  = 0x80
	ATA_STAT_DRDY = 0x40
	ATA_STAT_DRQ  = 0x08
	ATA_STAT_ERR  = 0x01

	ATA_CMD_IDENTIFY         = 0xEC
	ATA_CMD_READ             = 0x20
	ATA_CMD_READ_DMA         = 0xC8
	ATA_CMD_READ_SECTORS_EXT = 0x24
	ATA_CMD_SET_FEATURES     = 0xEF
	ATA_CMD_READ_NATIVE_MAX  = 0xF8
	ATA_CMD_IDLE_NEW         = 0xE3
	ATA_CMD_CHECK_POWER      = 0xE5
	ATA_CMD_IDLE             = 0x97
	ATA_CMD_STANDBY          = 0x96
	ATA_CMD_INIT_DEV_PARAMS  = 0x91

	BUS_INT_RIO_DEVICE1 = 1 << 3
)

type IDEController struct {
	File      *os.File
	FileSize  int64
	Asic      *SoloASIC
	ReadCount int64

	Status      uint8
	Error       uint8
	SectorCount uint8
	LBA         uint32
	DevHead     uint8
	Features    uint8

	DataBuf []byte
	DataPos int

	IdentifyData [512]byte
}

func NewIDEController(asic *SoloASIC) *IDEController {
	return &IDEController{
		Status: ATA_STAT_DRDY,
		Asic:   asic,
	}
}

func (ide *IDEController) Open(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	ide.File = f
	ide.FileSize = fi.Size()
	ide.buildIdentify()
	fmt.Printf("IDE: disk %d sectors (%d MB)\n", fi.Size()/512, fi.Size()/(1024*1024))
	return nil
}

func (ide *IDEController) buildIdentify() {
	id := ide.IdentifyData[:]
	binary.LittleEndian.PutUint16(id[0:], 0x0040)
	binary.LittleEndian.PutUint16(id[2:], 16383)
	binary.LittleEndian.PutUint16(id[6:], 16)
	binary.LittleEndian.PutUint16(id[12:], 63)
	copy(id[20:], []byte("WTVDISK001         "))
	copy(id[46:], []byte("1.0     "))
	copy(id[54:], []byte("WebTV HDD                               "))
	binary.LittleEndian.PutUint16(id[98:], 0x0F00)
	total := uint32(ide.FileSize / 512)
	binary.LittleEndian.PutUint16(id[120:], uint16(total))
	binary.LittleEndian.PutUint16(id[122:], uint16(total>>16))
	binary.LittleEndian.PutUint16(id[166:], 0x0400)
	binary.LittleEndian.PutUint16(id[172:], 0x0400)
	binary.LittleEndian.PutUint16(id[200:], uint16(total))
	binary.LittleEndian.PutUint16(id[202:], uint16(total>>16))
	binary.LittleEndian.PutUint16(id[204:], 0)
	binary.LittleEndian.PutUint16(id[206:], 0)
}

func (ide *IDEController) irq() {
	if ide.Asic != nil {
		ide.Asic.RIOIntStat |= BUS_INT_RIO_DEVICE1
		ide.Asic.assertBusIRQ()
	}
}

func (ide *IDEController) Read32(off uint32) uint32 {
	switch off {
	case 0x00:
		return ide.readData()
	case 0x04:
		return uint32(ide.Error)
	case 0x08:
		return uint32(ide.SectorCount)
	case 0x0C:
		return uint32(ide.LBA & 0xFF)
	case 0x10:
		return uint32((ide.LBA >> 8) & 0xFF)
	case 0x14:
		return uint32((ide.LBA >> 16) & 0xFF)
	case 0x18:
		return uint32(ide.DevHead)
	case 0x1C:
		return uint32(ide.Status)
	case 0x400018:
		return uint32(ide.Status)
	default:
		return 0
	}
}

func (ide *IDEController) Write32(off, val uint32) {
	switch off {
	case 0x04:
		ide.Features = uint8(val)
	case 0x08:
		ide.SectorCount = uint8(val)
	case 0x0C:
		ide.LBA = (ide.LBA & 0xFFFF00) | (val & 0xFF)
	case 0x10:
		ide.LBA = (ide.LBA & 0xFF00FF) | ((val & 0xFF) << 8)
	case 0x14:
		ide.LBA = (ide.LBA & 0x00FFFF) | ((val & 0xFF) << 16)
	case 0x18:
		ide.DevHead = uint8(val)
		ide.LBA = (ide.LBA & 0x0FFFFF) | ((uint32(val) & 0x0F) << 24)
	case 0x1C:
		ide.execCommand(uint8(val))
	case 0x400018:
		if val&0x04 != 0 {
			ide.Status = ATA_STAT_DRDY
			ide.Error = 0
		}
	}
}

func (ide *IDEController) readData() uint32 {
	if ide.DataBuf == nil || ide.DataPos+3 >= len(ide.DataBuf) {
		return 0xFFFFFFFF
	}
	lo := binary.LittleEndian.Uint16(ide.DataBuf[ide.DataPos:])
	hi := binary.LittleEndian.Uint16(ide.DataBuf[ide.DataPos+2:])
	ide.DataPos += 4
	if ide.DataPos >= len(ide.DataBuf) {
		ide.DataBuf = nil
		ide.Status = ATA_STAT_DRDY
	}
	return uint32(lo) | (uint32(hi) << 16)
}

func (ide *IDEController) execCommand(cmd uint8) {
	switch cmd {
	case ATA_CMD_IDENTIFY:
		ide.DataBuf = ide.IdentifyData[:]
		ide.DataPos = 0
		ide.Status = ATA_STAT_DRDY | ATA_STAT_DRQ
		ide.irq()

	case ATA_CMD_READ, ATA_CMD_READ_DMA:
		ide.doReadSectors()

	case ATA_CMD_SET_FEATURES:
		ide.Status = ATA_STAT_DRDY
		ide.irq()

	case ATA_CMD_READ_NATIVE_MAX:
		ide.Status = ATA_STAT_DRDY
		maxLBA := uint32(ide.FileSize/512 - 1)
		ide.LBA = maxLBA & 0x0FFFFFFF
		ide.DevHead = uint8((maxLBA>>24)&0x0F) | 0xE0
		ide.irq()

	case ATA_CMD_IDLE, ATA_CMD_IDLE_NEW, ATA_CMD_STANDBY:
		ide.Status = ATA_STAT_DRDY
		ide.irq()

	case ATA_CMD_CHECK_POWER:
		ide.SectorCount = 0xFF
		ide.Status = ATA_STAT_DRDY
		ide.irq()

	case ATA_CMD_INIT_DEV_PARAMS:
		ide.Status = ATA_STAT_DRDY
		ide.irq()

	default:
		fmt.Printf("IDE: unhandled cmd 0x%02X\n", cmd)
		ide.Error = 0x04
		ide.Status = ATA_STAT_DRDY | ATA_STAT_ERR
		ide.irq()
	}
}

func (ide *IDEController) doReadSectors() {
	ide.ReadCount++
	lba := ide.LBA & 0x0FFFFFFF
	count := uint32(ide.SectorCount)
	if count == 0 {
		count = 256
	}
	totalBytes := count * 512
	buf := make([]byte, totalBytes)
	if ide.File != nil {
		offset := int64(lba) * 512
		if offset+int64(totalBytes) > ide.FileSize {
			n := ide.FileSize - offset
			if n < 0 {
				n = 0
			}
			if n > int64(totalBytes) {
				n = int64(totalBytes)
			}
			ide.File.ReadAt(buf[:n], offset)
		} else {
			ide.File.ReadAt(buf, offset)
		}
	}
	ide.DataBuf = buf
	ide.DataPos = 0
	ide.Status = ATA_STAT_DRDY | ATA_STAT_DRQ
	ide.irq()
}
