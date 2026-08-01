package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadROMFSFile(t *testing.T) {
	data := make([]byte, 256)
	binary.BigEndian.PutUint32(data[0x24:], 0x1100)
	binary.BigEndian.PutUint32(data[0x30:], 0x1000)
	root := data[0xC0:]
	binary.BigEndian.PutUint32(root[8:], 0x1040)
	copy(root[28:], "ROM")
	file := data[0x40:]
	binary.BigEndian.PutUint32(file[4:], 0x10C0)
	binary.BigEndian.PutUint32(file[12:], 0x1080)
	binary.BigEndian.PutUint32(file[16:], 6)
	copy(file[28:], "logo.gif")
	copy(data[0x80:], "GIF89a")
	got, err := readROMFSFile(data, "/ROM/logo.gif")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("GIF89a")) {
		t.Fatalf("got %q", got)
	}
}
