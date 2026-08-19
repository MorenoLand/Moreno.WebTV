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
- SDL opens while CPU execution continues, presents the boot ROM's real gold splash, and switches to live YUY2 scanout from `VID_NSTART` only when a stable valid guest frame exists.

## Latest Verification (2026-08-03)

- A clean `WEBTV_STEPS=65000000 WEBTV_VIDEO=0 WEBTV_COMPAT_SHIMS=0 WEBTV_SYNTH_CLOCK=0 WEBTV_SYNTHETIC_PPP=0` run reaches `PC=0x80222E5C`, `Status=0x3400C401`, `ModemStage=4`, `294` modem reads, `128` modem writes, `0` received bytes, `0` IDE sector reads, and `VID_NSTART=0x80000000`.
- A full `650000000`-step run with those same flags reaches `PC=0x8022A448`, keeps gate byte `0x8000224E` at `0`, leaves only sentinel event key `0xFF`, and still performs `0` IDE sector reads.
- The old host-side connecting/road fallback is not part of the current viewer. SDL presents the ROM-backed splash and switches only to a stable guest YUY2 frame.

## Current Blocker

- The boot ROM directly contains `Connecting.html`, `ConnectingTo.gif`, `CityGlow.gif`, `CityRoad.gif`, `CityRoad.cel`, and `DialingWebTV.mid`; `ConnectSetup.html` contains the quick-connect setup menu. The guest has loaded `Connecting.html`, but has not reached the event/network path that makes the firmware render those resources.

- A normal 60-million-step run reaches `PC=0x802C60E4` after the modem download with `ModemStage=4`, `294` modem reads, `128` modem writes, `0` received bytes, `0` IDE sector reads, and `VID_NSTART=0x80000000`.
- A normal 650-million-step run remains stable at `PC=0x8022A448`, `Status=0x3400C401`; gate byte `0x8000224E` is `0`, the event list contains only sentinel key `0xFF`, and no live framebuffer is selected.
- A focused trace found the candidate event-producer callers executing, but no entry into the producer code at `0x80234E80..0x80234F80`. The missing transition is therefore still before event-key insertion, not an SDL presentation problem.
- A follow-up bus trace found the direct event-2 attempt: callback `0x802C73C0` calls enqueue `0x802C5E84` repeatedly, but the enqueue reads `A0000588` as `0x00000000` and rejects it at `0x802C5EBC` because the high word is not `0x7661`. No guest write to physical `0x400..0x58F` occurs after the reset sentinel fill, so this signature is supplied by an unmodeled device/ROM contract rather than a missed RAM constructor write.
- An expanded `WEBTV_TRACE_DEVICE_OBJECT=1` run shows the crash-signature read at `0x80222E38` returning `0x00000000`; the dumped `0x802C5D00` setup routine only clears `A0000588` at `0x802C5D44` and `0x802C5D80`. No observed path supplies `0x43727368` at `A0000400` or `0x7661` in the event field, so no value was synthesized.
- The relocated runtime dump shows the live event path calls `0x802C60DC` from `0x802C5EA0`; its `0x802C5FD4` callee operates on timer globals `0x80001CF0/0x1CF4/0x1CF8` and does not write the low-RAM object. The only direct references found to setup routine `0x802C5D00` are at `0x802C7664` and `0x802C766C`, and neither executes in the clean run.
- The synthetic PPP negative control queues a 12-byte LCP Configure-Request and receives one `0x7E` (`rx=1`, `wire=10`), but stops at `PC=0x802216C0` before the direct event enqueue reads `A0000588`; the event table remains sentinel-only. This does not identify a PPP peer as the missing status source.
- A receive-window trace localizes that negative control further: RIO bit `0x04` enters the UART IRQ at `0x8022168C`, the RX handler reads `0x7E` at step `57877403`, and it calls the packet parser at `0x80243480`. The parser immediately reads `0x80001A3C == 0` and returns through its null-input branch; no packet state, event-2 callback, IDE read, or guest video follows. The remaining synthetic bytes are therefore blocked at the uninitialized receive-queue state, not at UART/RIO routing.
- A bounded `WEBTV_TRACE_PACKET_QUEUE=1` capture shows the boot ROM first seeds `0x80001A3C..0x80001A4B` with `0xC0EDBABE`, then firmware routine `0x8024E79C` clears that range byte-by-byte to zero around steps `29739179..29739361`. Neither clean nor synthetic execution records a later nonzero write; the synthetic parser read at step `57877414` therefore observes the firmware-initialized null queue pointer. The tracer is opt-in and does not change the normal path.
- Static immediate references identify `0x80243484/0x8024349C` as the active parser loads of `0x1A3C/0x1A40`, with dormant consumers at `0x80243F80/0x80243F88`; adjacent `0x1A44/0x1A48` loads occur in the parser family at `0x80243440/0x8024344C` and `0x8024385C/0x80243864`. No direct immediate store producer for these globals appears in the scan, leaving the network/device queue producer unresolved.
- Disabling synthetic carrier assertion is a negative control: the guest stops at `PC=0x80221608` with only `288` modem reads and `127` writes, never reaching the post-carrier wait. The carrier-enabled model remains the verified baseline.

- The external WebTV/Pokoe probe remains unmapped by default; no host response is injected. A clean 65-million-step run therefore remains at the authentic post-carrier wait with `VID_NSTART=0x80000000`, `0` IDE reads, and no guest-rendered connecting frame.
- Missing MIPS III `REGIMM` branch-likely and immediate-trap instructions caused the image manager to initialize out of order. With those instructions implemented, object field `0x800EADB4` receives `0x800EB298` and the old `0x802E80D8` null dereference is gone.
- The firmware registers UART interrupt ID `0x14`, enables SOLO RIO register `0xA4000094` bit `0x04`, and services it at `0x8022168C`. Routing UART through RIO parent bit `0x20` reaches that handler, which drains IIR/MSR and exits cleanly; `0xA4004028` is the unrelated IR status register.
- The authentic UART carrier interrupt path now completes, but boot event gate byte `0x8000224E` remains unset. At `0x802C6828`, firmware queries the list rooted at `0x800017A0` for event key `2`; the live list contains only sentinel key `0xFF`, so the gate setter is never called. There is no in-process PPP/WTVP peer supplying the post-carrier receive bytes or network event; the optional TCP modem bridge only forwards serial traffic.
- Forcing the gate setter is diagnostic only: it advances the guest into its UART receive ring, but still produces no IDE reads or valid live frame. It must remain disabled for authentic boot verification.
- `Connecting.html` is loaded successfully before this wait. Forcing the pending scheduler timestamp executes callback `0x8022A7A4` repeatedly but does not create event key `2`, ruling out a simple overdue-timer correction; the next blocker is the missing event producer.
- The normal 650-million-step run completes without a CPU error, with `EXL=0` and no pending bus interrupt, but it does not select a live framebuffer. The packed YUY2 geometry and live SDL scanout remain validated only on the forced diagnostic branch, which must stay disabled for authentic boot verification.
- The gold logo shown by SDL is decoded directly from `/ROM/Images/SplashLogo1.gif`; it is a ROM-backed presentation asset, not proof that the guest rendered the current page. No host-generated connecting/road fallback remains.
