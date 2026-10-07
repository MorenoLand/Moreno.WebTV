package main

func remoteIRTransitions(command byte) []uint16 {
	if command > 0x7F {
		return nil
	}
	word := uint32(0x79D00) | uint32(command)
	transitions := make([]uint16, 1, 85)
	transitions[0] = 0x0BE8
	for frame := 0; frame < 2; frame++ {
		transitions = append(transitions, 0x00A0, 0x0828)
		for bit := 0; bit < 20; bit++ {
			duration := uint16(40)
			if word&(1<<bit) != 0 {
				duration = 80
			}
			transitions = append(transitions, duration)
			if bit != 19 {
				transitions = append(transitions, 0x0828)
			}
		}
		transitions = append(transitions, 0x0BE8)
	}
	return transitions
}
