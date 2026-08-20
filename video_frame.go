package main

const videoWidth = 560
const videoHeight = 420

func videoFrameBytes(bus *Bus) ([]byte, bool) {
	length := videoWidth * videoHeight * 2
	if bus.Asic.RefreshVideoDMA() && len(bus.Asic.VideoFrame) >= length {
		return bus.Asic.VideoFrame[:length], true
	}
	base := bus.Asic.VIDNStart & 0x1FFFFFFF
	if base == 0 || uint64(base)+uint64(length) > uint64(len(bus.RAM)) {
		return nil, false
	}
	return bus.RAM[base : base+uint32(length)], true
}
