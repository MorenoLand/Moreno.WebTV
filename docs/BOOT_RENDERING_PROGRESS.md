# Boot And Rendering Progress

## Confirmed Image Layout

- The app build header reports load address `0x804E0000` and ROMFS address `0x80800000`.
- The build length field excludes its first `0x0C` bytes. The loader must copy `romLen + 0x0C`, or the final twelve bytes of the in-memory build are lost.
- ROMFS is part of the loaded build, after the executable code.

## Current Boot Path

- Boot ROM and app firmware execute through the WebTV resource loader.
- The firmware resolves ROM resources and reaches image resources with `GIFf` headers.
- The display scanout is configured at `VID_NSTART = 0x8000B740`.
- The boot ROM ROMFS contains the authentic startup animation descriptor at `/ROM/Animations/SplashLogo.cel`. It dissolves from `/ROM/Images/SplashLogo2.gif` to the gold `/ROM/Images/SplashLogo1.gif`; both are `232x174`.
- The app ROM animation `/ROM/Animations/WebTVLogo.cel` references the boot ROM cache asset `/ROM/Cache/WebTVLogoJewel.gif`, which is `141x109`.
- SDL now opens while CPU execution continues, presents the boot ROM's real gold splash for the initial four seconds, then switches to live YUY2 scanout from `VID_NSTART`.

## Current Blocker

- Missing MIPS III `REGIMM` branch-likely and immediate-trap instructions caused the image manager to initialize out of order. With those instructions implemented, object field `0x800EADB4` receives `0x800EB298` and the old `0x802E80D8` null dereference is gone.
- The modem RX path asserted SOLO bus bit `0x40`, which firmware dispatches as audio through status register `0xA4000068`. Firmware's own interrupt table maps device status `0xA4000074` to bus bit `0x10`; correcting that mapping clears the later interrupt-dispatch loop.
- A full 650-million-step run now completes without a CPU error at normal firmware PC `0x802D4FC8`, with `EXL=0` and no pending bus interrupt.
- `VID_NSTART` points to a packed YUY2 frame. Its first scanline is 1120 bytes and video register `0x3010` contains `0x72D80`, exactly `560 * 420 * 2`; SDL scanout now uses the firmware's `560x420` geometry.
- The corrected final framebuffer renders the authentic WebTV Plus `Remove this printer adapter` screen, including the WebTV logo and firmware text. The firmware-to-SDL video path is now proven end to end.
