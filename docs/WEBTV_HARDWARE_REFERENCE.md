# WebTV Hardware Reference

Working notes distilled from the WebTV Wiki. These are reference facts, not a substitute for the full SOLO1 specification.

## Addressing

- Reset begins at `0xBFC00000`; the physical boot ROM mirror is `0x9FC00000`.
- The SOLO/SOLO2 aperture begins at `0xA4000000`; register offsets are relative to that base.
- `0xA400300C` is the video frame-buffer start register (`VID_NSTART`).
- `0xA4000048` is the high-resolution clock and `0xA40000A8` is the crash counter.
- The firmware enables the WebTV Port through `0xA400101C = 0x31`.

## Firmware Layout

- Boot ROMs are 2 MiB on the non-Japanese hardware and are interleaved in 16-bit pieces across ROM chips.
- Boot ROM code initializes the machine, loads a mini browser, and then transfers to the main client firmware.
- The ROM header is big-endian and starts with a jump instruction followed by checksums, lengths, load address, and build flags.

## Current Emulator Evidence

- Firmware writes `VID_NSTART=0x8000B740` and enables video interrupt bit `0x8`.
- The emulator now supplies a paced video interrupt and tracks acknowledgement through the SOLO video interrupt registers.
- Firmware's bus interrupt table maps video/audio/RIO/device/timer to bits `0x80/0x40/0x20/0x10/0x08` respectively.
- Video register `0x3010` contains frame size `0x72D80`; together with the 1120-byte scanline this identifies the packed YUY2 scanout as `560x420`.
- The former image-manager null dereference at `0x802E80D8` and the later audio-dispatch loop are fixed; a full run now remains in normal firmware execution with `EXL=0`.

## Sources

- https://wiki.webtv.zone/wiki/WebTV_Memory_Map
- https://wiki.webtv.zone/wiki/Solo
- https://wiki.webtv.zone/wiki/ROM
- https://bitsavers.computerhistory.org/pdf/webTV/SOLO1_ASIC_Spec.pdf
