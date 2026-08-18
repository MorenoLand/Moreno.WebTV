package main

import "testing"

func TestPPPLCPConfigureRequestFrame(t *testing.T) {
	frame := pppLCPConfigureRequest(1)
	if len(frame) != 12 || frame[0] != pppFlag || frame[len(frame)-1] != pppFlag {
		t.Fatalf("LCP frame shape=% X", frame)
	}
	data := make([]byte, 0, len(frame)-2)
	escape := false
	for _, value := range frame[1 : len(frame)-1] {
		if escape {
			data = append(data, value^pppEscapeX)
			escape = false
		} else if value == pppEscape {
			escape = true
		} else {
			data = append(data, value)
		}
	}
	if escape || len(data) != 10 || data[0] != 0xFF || data[1] != 0x03 || data[2] != 0xC0 || data[3] != 0x21 || data[4] != 1 || data[5] != 1 || data[6] != 0 || data[7] != 4 {
		t.Fatalf("LCP frame data=% X", data)
	}
	if pppFCS(data[:8]) != uint16(data[8])|uint16(data[9])<<8 {
		t.Fatalf("LCP FCS=%02X%02X want=%04X", data[9], data[8], pppFCS(data[:8]))
	}
}
