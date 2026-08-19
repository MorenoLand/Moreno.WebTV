package main

import "fmt"

type modemSRecord struct {
	kind    byte
	address uint32
	data    []byte
}

func parseModemSRecord(line []byte) (modemSRecord, error) {
	if len(line) < 6 || line[0] != 'S' || line[1] < '0' || line[1] > '9' {
		return modemSRecord{}, fmt.Errorf("invalid S-record header")
	}
	count, err := modemHexByte(line[2:4])
	if err != nil {
		return modemSRecord{}, err
	}
	if len(line) != 4+int(count)*2 {
		return modemSRecord{}, fmt.Errorf("S-record length %d does not match count %d", len(line), count)
	}
	addressBytes := 0
	switch line[1] {
	case '0', '1', '5', '9':
		addressBytes = 2
	case '2', '6', '8':
		addressBytes = 3
	case '3', '7':
		addressBytes = 4
	default:
		return modemSRecord{}, fmt.Errorf("unsupported S-record type S%c", line[1])
	}
	if int(count) < addressBytes+1 {
		return modemSRecord{}, fmt.Errorf("S-record S%c has no checksum-sized payload", line[1])
	}
	bytes := make([]byte, count)
	for i := range bytes {
		value, err := modemHexByte(line[4+i*2 : 6+i*2])
		if err != nil {
			return modemSRecord{}, err
		}
		bytes[i] = value
	}
	checksum := byte(0)
	for _, value := range append([]byte{count}, bytes...) {
		checksum += value
	}
	if checksum != 0xFF {
		return modemSRecord{}, fmt.Errorf("S-record checksum 0x%02X", checksum)
	}
	var address uint32
	for _, value := range bytes[:addressBytes] {
		address = address<<8 | uint32(value)
	}
	data := append([]byte(nil), bytes[addressBytes:len(bytes)-1]...)
	return modemSRecord{kind: line[1], address: address, data: data}, nil
}

func modemHexByte(value []byte) (byte, error) {
	if len(value) != 2 {
		return 0, fmt.Errorf("invalid S-record hex byte")
	}
	upper := func(value byte) (byte, bool) {
		switch {
		case value >= '0' && value <= '9':
			return value - '0', true
		case value >= 'A' && value <= 'F':
			return value - 'A' + 10, true
		case value >= 'a' && value <= 'f':
			return value - 'a' + 10, true
		default:
			return 0, false
		}
	}
	hi, ok := upper(value[0])
	if !ok {
		return 0, fmt.Errorf("invalid S-record hex byte")
	}
	lo, ok := upper(value[1])
	if !ok {
		return 0, fmt.Errorf("invalid S-record hex byte")
	}
	return hi<<4 | lo, nil
}
