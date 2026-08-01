package main

const videoWidth = 560
const videoHeight = 420

func videoFrameBytes(bus *Bus) ([]byte, bool) {
	base := bus.Asic.VIDNStart & 0x1FFFFFFF
	length := videoWidth * videoHeight * 2
	if base == 0 || uint64(base)+uint64(length) > uint64(len(bus.RAM)) {
		return nil, false
	}
	return bus.RAM[base : base+uint32(length)], true
}
