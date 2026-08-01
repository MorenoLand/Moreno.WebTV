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

- Early image-resource construction reaches `0x8026C1D8` before its decoder registry at `0x80001E20` has been initialized, so it returns null.
- Firmware stores that null in object field `0x800EACA4 + 0x110`, then later dereferences it at `0x802E80D8`.
- The registry is initialized later by firmware routine `0x8026C288`. The next task is to trace its caller and correct the scheduler or initialization ordering, rather than synthesize a decoder object.
- The initializer is called from the resource completion path at `0x8026D710`; its first observed installation occurs at step `47,397,638`, after the affected early-resource calls.
- The earlier gate is global manager pointer `0x80002658`: it remains zero during the first `GIFf` factory calls at step `45,620,423`, then constructor `0x802E8D0C` installs `0x800EBD78` at step `47,029,614` from startup allocator `0x802F0D1C`.
- Startup enters the GIF loader reentrantly while constructing earlier managers. Firmware records the null result in object field `0x800EADB4`, but no later write backfills that field before its dereference.
- The first observed scheduler callback does not dispatch until step `50,424,007`, after the manager and registry are valid. The next task is to trace why the deferred resource completion that owns `0x800EADB4` is absent or dropped.
- A controlled run past the first null reaches a second null at `0x8026A0C0` without changing the framebuffer, confirming that the registry ordering is the upstream blocker.
- SOLO timer source enables now route through `BUS_INT_TIMER` without inventing source bit 3 or bypassing the bus interrupt mask. Upstream arms its 50 ms callback only after a nonzero `TCOMPARE`; this firmware does not write `TCOMPARE`, so delivering that callback from `TIMINTEN` alone is invalid.
- The `0xA40090A0` RIO reads are confined to boot calibration and end near step `4.57M`. No RIO reads occur near the resource-ordering failure, so the synthetic late-cycle return is dormant and is not the current blocker.
