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

func TestPPPDecoderAcceptsEscapedFrame(t *testing.T) {
	frame := pppFrame(pppLCP, []byte{1, 0x7D, 0x7E, 0})
	var decoder pppDecoder
	packets := decoder.Push(frame[:5])
	packets = append(packets, decoder.Push(frame[5:])...)
	if len(packets) != 1 || packets[0].Protocol != pppLCP {
		t.Fatalf("decoded packets=%+v", packets)
	}
	if got := packets[0].Information; len(got) != 4 || got[1] != 0x7D || got[2] != 0x7E {
		t.Fatalf("decoded information=% X", got)
	}
}

func TestPPPDecoderRejectsBadFCSAndAcceptsCompressedHeader(t *testing.T) {
	frame := pppFrame(pppLCP, []byte{1, 2, 0, 4})
	frame[len(frame)-2] ^= 1
	var decoder pppDecoder
	if packets := decoder.Push(frame); len(packets) != 0 {
		t.Fatalf("bad FCS decoded=%+v", packets)
	}
	data := []byte{0x21, 1, 2, 0, 4}
	fcs := pppFCS(data)
	compressed := []byte{pppFlag, 0x21, 1, 2, 0, 4, byte(fcs), byte(fcs >> 8), pppFlag}
	decoder = pppDecoder{}
	if packets := decoder.Push(compressed); len(packets) != 1 || packets[0].Protocol != 0x21 {
		t.Fatalf("compressed header decoded=%+v", packets)
	}
}

func TestBusTracksFragmentedIncomingPPPFrame(t *testing.T) {
	bus := NewBus()
	frame := pppLCPConfigureRequest(2)
	bus.queueModemWireRX(frame[:3])
	if bus.ModemPPPFrames != 0 {
		t.Fatalf("partial frame count=%d", bus.ModemPPPFrames)
	}
	bus.queueModemWireRX(frame[3:])
	if bus.ModemPPPFrames != 1 || bus.ModemLastPPPProtocol != pppLCP || len(bus.ModemLastPPPInfo) != 4 {
		t.Fatalf("decoded frames=%d protocol=0x%04X info=% X", bus.ModemPPPFrames, bus.ModemLastPPPProtocol, bus.ModemLastPPPInfo)
	}
}
