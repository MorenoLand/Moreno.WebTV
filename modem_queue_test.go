package main

import "testing"

func TestModemQueueProbeSeedsObservedQueueWords(t *testing.T) {
	t.Setenv("WEBTV_MODEM_QUEUE_PROBE", "0x807F0000,0x2")
	bus := NewBus()
	bus.queueModemWireRX(pppLCPConfigureRequest(1))
	if !bus.ModemQueueProbeSeeded {
		t.Fatal("PPP frame did not seed the queue probe")
	}
	if got := bus.Read32(0x80001A3C); got != 0x807F0000 {
		t.Fatalf("queue pointer=0x%08X, want 0x807F0000", got)
	}
	if got := bus.Read32(0x80001A40); got != 2 {
		t.Fatalf("queue mask=0x%08X, want 2", got)
	}
	if got := bus.Read32(0x807F000C); got != 0x4 {
		t.Fatalf("queue flags=0x%08X, want 4", got)
	}
	if got := bus.Read32(0x807F0004); got != 0x807F0040 {
		t.Fatalf("queue target=0x%08X, want 0x807F0040", got)
	}
	if got := bus.Read32(0x807F0018); got != 2 {
		t.Fatalf("queue mask field=0x%08X, want 2", got)
	}
}
