package main

import (
	"net"
	"testing"
	"time"
)

func TestModemTCPLinkMovesBytes(t *testing.T) {
	left, right := net.Pipe()
	link := newModemTCPLink(left)
	defer link.close()
	defer right.Close()
	go func() {
		_, _ = right.Write([]byte{0x7E, 0x01})
	}()
	deadline := time.Now().Add(time.Second)
	var got []byte
	for time.Now().Before(deadline) && len(got) < 2 {
		got = append(got, link.poll()...)
		time.Sleep(time.Millisecond)
	}
	if len(got) != 2 || got[0] != 0x7E || got[1] != 0x01 {
		t.Fatalf("peer RX=% X, want 7E 01", got)
	}
	if !link.send([]byte{0x7E, 0x02}) {
		t.Fatal("link rejected TX")
	}
	_ = right.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 2)
	n, err := right.Read(buf)
	if err != nil || n != 2 || buf[0] != 0x7E || buf[1] != 0x02 {
		t.Fatalf("peer TX=% X err=%v, want 7E 02", buf[:n], err)
	}
}
