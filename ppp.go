package main

const (
	pppFlag    = 0x7E
	pppEscape  = 0x7D
	pppEscapeX = 0x20
	pppLCP     = 0xC021
)

type pppPacket struct {
	Protocol    uint16
	Information []byte
}

type pppDecoder struct {
	data    []byte
	escaped bool
	active  bool
}

func pppFCS(data []byte) uint16 {
	fcs := uint16(0xFFFF)
	for _, value := range data {
		fcs ^= uint16(value)
		for bit := 0; bit < 8; bit++ {
			if fcs&1 != 0 {
				fcs = fcs>>1 ^ 0x8408
			} else {
				fcs >>= 1
			}
		}
	}
	return ^fcs
}

func pppFrame(protocol uint16, information []byte) []byte {
	data := make([]byte, 0, len(information)+6)
	data = append(data, 0xFF, 0x03, byte(protocol>>8), byte(protocol))
	data = append(data, information...)
	fcs := pppFCS(data)
	data = append(data, byte(fcs), byte(fcs>>8))
	frame := []byte{pppFlag}
	for _, value := range data {
		if value == pppFlag || value == pppEscape {
			frame = append(frame, pppEscape, value^pppEscapeX)
		} else {
			frame = append(frame, value)
		}
	}
	return append(frame, pppFlag)
}

func (d *pppDecoder) Push(data []byte) []pppPacket {
	var packets []pppPacket
	for _, value := range data {
		switch value {
		case pppFlag:
			if d.active {
				if packet, ok := decodePPPData(d.data); ok {
					packets = append(packets, packet)
				}
			}
			d.data = d.data[:0]
			d.escaped = false
			d.active = true
		case pppEscape:
			if d.active {
				d.escaped = true
			}
		case pppEscapeX:
			if d.active && d.escaped {
				d.data = append(d.data, value^pppEscapeX)
				d.escaped = false
			} else if d.active {
				d.data = append(d.data, value)
			}
		default:
			if d.active {
				if d.escaped {
					d.data = append(d.data, value^pppEscapeX)
					d.escaped = false
				} else {
					d.data = append(d.data, value)
				}
			}
		}
	}
	return packets
}

func decodePPPData(data []byte) (pppPacket, bool) {
	if len(data) < 4 {
		return pppPacket{}, false
	}
	if len(data) < 6 || pppFCS(data[:len(data)-2]) != uint16(data[len(data)-2])|uint16(data[len(data)-1])<<8 {
		return pppPacket{}, false
	}
	data = data[:len(data)-2]
	offset := 0
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0x03 {
		offset = 2
	}
	if len(data)-offset < 2 {
		return pppPacket{}, false
	}
	protocol := uint16(data[offset])
	offset++
	if protocol&1 == 0 {
		if len(data)-offset < 1 {
			return pppPacket{}, false
		}
		protocol = protocol<<8 | uint16(data[offset])
		offset++
	}
	return pppPacket{Protocol: protocol, Information: append([]byte(nil), data[offset:]...)}, true
}

func pppLCPConfigureRequest(identifier byte) []byte {
	return pppFrame(pppLCP, []byte{1, identifier, 0, 4})
}
