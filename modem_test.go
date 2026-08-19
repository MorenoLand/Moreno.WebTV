package main

import (
	"encoding/hex"
	"testing"
)

func TestParseModemSRecords(t *testing.T) {
	cases := []struct {
		line    string
		kind    byte
		address uint32
		data    string
	}{
		{"S308000002004C0302A4", '3', 0x200, "4c0302"},
		{"S3150000020378B2D533B2771FD2801A06D2011F066C95", '3', 0x203, "78b2d533b2771fd2801a06d2011f066c"},
		{"S30700000213FEFFE6", '3', 0x213, "feff"},
		{"S70500000000FA", '7', 0, ""},
	}
	for _, test := range cases {
		record, err := parseModemSRecord([]byte(test.line))
		if err != nil {
			t.Fatalf("parse %s: %v", test.line, err)
		}
		if record.kind != test.kind || record.address != test.address || hex.EncodeToString(record.data) != test.data {
			t.Fatalf("parse %s = S%c 0x%X %X", test.line, record.kind, record.address, record.data)
		}
	}
}

func TestParseModemSRecordRejectsBadChecksum(t *testing.T) {
	if _, err := parseModemSRecord([]byte("S70500000000FB")); err == nil {
		t.Fatal("bad S-record checksum was accepted")
	}
}

func TestModemUploadRequiresValidTerminator(t *testing.T) {
	bus := NewBus()
	bus.ModemStage = 1
	for _, line := range []string{"S308000002004C0302A4", "S3150000020378B2D533B2771FD2801A06D2011F066C95", "S30700000213FEFFE6"} {
		for i := 0; i < len(line); i++ {
			bus.modemWrite(0, uint32(line[i]), 1)
		}
		bus.modemWrite(0, '\r', 1)
	}
	bus.modemWrite(0, 'S', 1)
	bus.modemWrite(0, '7', 1)
	bus.modemWrite(0, '0', 1)
	bus.modemWrite(0, '5', 1)
	bus.modemWrite(0, '0', 1)
	bus.modemWrite(0, '0', 1)
	bus.modemWrite(0, '0', 1)
	bus.modemWrite(0, '0', 1)
	bus.modemWrite(0, '0', 1)
	bus.modemWrite(0, '0', 1)
	bus.modemWrite(0, 'F', 1)
	bus.modemWrite(0, 'B', 1)
	bus.modemWrite(0, '\r', 1)
	if bus.ModemStage != 1 || bus.ModemFirmwareReady || bus.ModemFirmwareRecords != 3 || bus.ModemFirmwareBytes != 21 {
		t.Fatalf("invalid terminator changed upload state: stage=%d ready=%v records=%d bytes=%d", bus.ModemStage, bus.ModemFirmwareReady, bus.ModemFirmwareRecords, bus.ModemFirmwareBytes)
	}
	for _, line := range []string{"S70500000000FA"} {
		for i := 0; i < len(line); i++ {
			bus.modemWrite(0, uint32(line[i]), 1)
		}
		bus.modemWrite(0, '\r', 1)
	}
	if bus.ModemStage != 3 || !bus.ModemFirmwareReady || len(bus.ModemRX) != 1 || bus.ModemSRAM[0x200] != 0x4C || bus.ModemSRAM[0x213] != 0xFE {
		t.Fatalf("valid terminator did not complete upload: stage=%d ready=%v rx=%d", bus.ModemStage, bus.ModemFirmwareReady, len(bus.ModemRX))
	}
	if bus.Read8(0x1E000200) != 0x4C || bus.Read32(0x1E000200) != 0x4C {
		t.Fatalf("modem SRAM mapping read8=%02X read32=%08X", bus.Read8(0x1E000200), bus.Read32(0x1E000200))
	}
}
