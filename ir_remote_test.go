package main

import "testing"

func TestRemotePowerTransitionFrames(t *testing.T) {
	transitions := remoteIRTransitions(0x2E)
	if len(transitions) != 127 || transitions[0] != 0x0BE8 {
		t.Fatalf("IR framing: %v", transitions)
	}
	for frame := 0; frame < 3; frame++ {
		start := 1 + frame*42
		if transitions[start] != 0x00A0 || transitions[start+1] != 0x0828 || transitions[start+41] != 0x0AF8 {
			t.Fatal("invalid remote frame boundaries")
		}
		word := uint32(0)
		for bit := 0; bit < 20; bit++ {
			pulse := transitions[start+2+bit*2]
			if pulse != 40 && pulse != 80 {
				t.Fatalf("invalid pulse: %#x", pulse)
			}
			if pulse == 80 {
				word |= 1 << bit
			}
			if bit != 19 && transitions[start+3+bit*2] != 0x0828 {
				t.Fatal("invalid bit spacing")
			}
		}
		if word != 0x79D2E {
			t.Fatalf("Power remote word=%#x, want 0x79D2E", word)
		}
		period := uint16(0)
		for _, transition := range transitions[start : start+42] {
			period += transition & 0x7FF
		}
		if period != 3000 {
			t.Fatalf("frame period=%d samples, want 45ms", period)
		}
	}
	if got := remoteIRTransitions(0x80); len(got) != 0 {
		t.Fatal("invalid remote command accepted")
	}
}

func TestRemotePowerFramesReachThresholdInterrupt(t *testing.T) {
	a := NewSoloASIC()
	a.Write(0x4020, 1185)
	a.Write(0x402C, 0x3083)
	a.Write(0x007C, 4)
	a.Write(0x000C, BUS_INT_DEVICE)
	transitions := remoteIRTransitions(0x2E)
	a.QueueIRTransitions(transitions)
	delivered := make([]uint16, 0, len(transitions))
	pairedAt88 := false
	for i, transition := range transitions {
		a.Tick(uint64(transition&0x7FF) * 1185)
		if !a.PendingIP2() {
			continue
		}
		for a.IRCount != 0 {
			delivered = append(delivered, uint16(a.Read(0x4028))&0x0FFF)
		}
		a.Write(0x0174, 4)
		if i == 87 {
			if len(delivered) < 85 {
				t.Fatalf("only %d transitions delivered at word 88", len(delivered))
			}
			for j, want := range transitions[:85] {
				if delivered[j] != want {
					t.Fatalf("paired remote frame transition %d changed", j)
				}
			}
			pairedAt88 = true
		}
	}
	if !pairedAt88 {
		t.Fatal("required frame pair did not arrive at word 88")
	}
	if len(delivered) < 85 {
		t.Fatalf("only %d transitions delivered before IRQ threshold", len(delivered))
	}
	for i, transition := range transitions[:85] {
		if delivered[i] != transition {
			t.Fatalf("paired remote frame transition %d changed", i)
		}
	}
}
