package main

const (
	pppFlag    = 0x7E
	pppEscape  = 0x7D
	pppEscapeX = 0x20
	pppLCP     = 0xC021
)

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

func pppLCPConfigureRequest(identifier byte) []byte {
	return pppFrame(pppLCP, []byte{1, identifier, 0, 4})
}
