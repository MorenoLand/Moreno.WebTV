package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

func readROMFSFile(data []byte, wanted string) ([]byte, error) {
	if len(data) < 0x44 {
		return nil, fmt.Errorf("firmware header is truncated")
	}
	buildAddress := binary.BigEndian.Uint32(data[0x30:])
	romfsAddress := binary.BigEndian.Uint32(data[0x24:])
	if romfsAddress < buildAddress+64 {
		return nil, fmt.Errorf("invalid ROMFS address 0x%08X", romfsAddress)
	}
	visited := make(map[uint32]bool)
	var walk func(uint32, uint32, string, bool) ([]byte, error)
	walk = func(address, parent uint32, path string, base bool) ([]byte, error) {
		for address != 0 {
			if visited[address] {
				return nil, fmt.Errorf("ROMFS node cycle at 0x%08X", address)
			}
			visited[address] = true
			if address < buildAddress || uint64(address-buildAddress)+56 > uint64(len(data)) {
				return nil, fmt.Errorf("ROMFS node 0x%08X is outside firmware", address)
			}
			off := int(address - buildAddress)
			node := data[off : off+56]
			name := node[28:56]
			if end := bytes.IndexByte(name, 0); end >= 0 {
				name = name[:end]
			}
			next := binary.BigEndian.Uint32(node[0:])
			nodeParent := binary.BigEndian.Uint32(node[4:])
			child := binary.BigEndian.Uint32(node[8:])
			dataAddress := binary.BigEndian.Uint32(node[12:])
			size := binary.BigEndian.Uint32(node[16:])
			nodePath := path + "/" + string(name)
			if nodeParent == parent {
				if strings.EqualFold(nodePath, wanted) {
					if size&0xF0000000 != 0 {
						return nil, fmt.Errorf("%s is compressed", wanted)
					}
					dataOff, dataSize := uint64(dataAddress-buildAddress), uint64(size&0x00FFFFFF)
					if dataAddress < buildAddress || dataOff+dataSize > uint64(len(data)) {
						return nil, fmt.Errorf("%s data is outside firmware", wanted)
					}
					return data[dataOff : dataOff+dataSize], nil
				}
				if child != 0 {
					result, err := walk(child, address, nodePath, false)
					if err != nil || result != nil {
						return result, err
					}
				}
			}
			if base {
				break
			}
			address = next
		}
		return nil, nil
	}
	result, err := walk(romfsAddress-64, 0, "", true)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("%s was not found", wanted)
	}
	return result, nil
}
