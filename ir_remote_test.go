package main

import "testing"

func TestRemotePowerTransitionFrames(t *testing.T) {
	transitions := remoteIRTransitions(0x2E)
	if len(transitions) != 85 || transitions[0] != 0x0BE8 {
		t.Fatalf("IR framing: %v", transitions)
	}
	for frame := 0; frame < 2; frame++ {
		start := 1 + frame*42
		if transitions[start] != 0x00A0 || transitions[start+1] != 0x0828 || transitions[start+41] != 0x0BE8 {
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
	}
	if got := remoteIRTransitions(0x80); len(got) != 0 {
		t.Fatal("invalid remote command accepted")
	}
}
