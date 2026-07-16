package main

const videoWidth = 640
const videoHeight = 400

func videoFrameBytes(bus *Bus) ([]byte, bool) {
	base := bus.Asic.VIDNStart & 0x1FFFFFFF
	length := videoWidth * videoHeight * 2
	if uint64(base)+uint64(length) > uint64(len(bus.RAM)) {
		return nil, false
	}
	return bus.RAM[base : base+uint32(length)], true
}
