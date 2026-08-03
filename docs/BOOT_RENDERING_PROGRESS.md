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

## Latest Verification (2026-08-03)

## Current Blocker

- A normal 60-million-step run reaches `PC=0x802C60E4` after the modem download with `ModemStage=4`, `294` modem reads, `128` modem writes, `0` received bytes, `0` IDE sector reads, and `VID_NSTART=0x80000000`.
- A normal 650-million-step run remains stable at `PC=0x8022A448`, `Status=0x3400C401`; gate byte `0x8000224E` is `0`, the event list contains only sentinel key `0xFF`, and no live framebuffer is selected.
- A focused trace found the candidate event-producer callers executing, but no entry into the producer code at `0x80234E80..0x80234F80`. The missing transition is therefore still before event-key insertion, not an SDL presentation problem.
- Disabling synthetic carrier assertion is a negative control: the guest stops at `PC=0x80221608` with only `288` modem reads and `127` writes, never reaching the post-carrier wait. The carrier-enabled model remains the verified baseline.

- The default SOLO parallel-port status is now zero, so the WebTV Plus firmware passes its external-printer-adapter check. A clean 60-million-step run reaches the post-download modem wait with no adapter error, `ModemStage=4`, `294` modem reads, `128` modem writes, and zero IDE sector reads.
- Missing MIPS III `REGIMM` branch-likely and immediate-trap instructions caused the image manager to initialize out of order. With those instructions implemented, object field `0x800EADB4` receives `0x800EB298` and the old `0x802E80D8` null dereference is gone.
- The firmware registers UART interrupt ID `0x14`, enables SOLO RIO register `0xA4000094` bit `0x04`, and services it at `0x8022168C`. Routing UART through RIO parent bit `0x20` reaches that handler, which drains IIR/MSR and exits cleanly; `0xA4004028` is the unrelated IR status register.
- The authentic UART carrier interrupt path now completes, but boot event gate byte `0x8000224E` remains unset. At `0x802C6828`, firmware queries the list rooted at `0x800017A0` for event key `2`; the live list contains only sentinel key `0xFF`, so the gate setter is never called. There is no in-process PPP/WTVP peer supplying the post-carrier receive bytes or network event; the optional TCP modem bridge only forwards serial traffic.
- Forcing the gate setter is diagnostic only: it advances the guest into its UART receive ring, but still produces no IDE reads or valid live frame. It must remain disabled for authentic boot verification.
- `Connecting.html` is loaded successfully before this wait. Forcing the pending scheduler timestamp executes callback `0x8022A7A4` repeatedly but does not create event key `2`, ruling out a simple overdue-timer correction; the next blocker is the missing event producer.
- The normal 650-million-step run completes without a CPU error, with `EXL=0` and no pending bus interrupt, but it does not select a live framebuffer. The packed YUY2 geometry and live SDL scanout remain validated only on the forced diagnostic branch, which must stay disabled for authentic boot verification.
