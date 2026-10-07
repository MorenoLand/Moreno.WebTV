# WebTV Plus Emulator

Standalone Go emulator workbench for WebTV Plus preservation.

The current target is a WebTV Plus-style machine with a SOLO ASIC, R4640 MIPS III CPU, 8 MB RAM, boot ROM mapped at `0xBFC00000`, firmware flash at `0x1F000000`, and an eventual SDL2 video/input frontend plus WTVP server connectivity.

This repo is still bring-up/debug code. It currently contains the CPU interpreter, memory bus, SOLO ASIC stub, IDE controller, I2C EEPROM model, tests, and firmware boot instrumentation.

## Current State

The emulator boots real boot ROM code and reaches firmware code paths. Recent work fixed several core correctness issues:

- ROM bounds checks no longer overflow on bad physical addresses.
- ERL-mode KSEG2 direct physical mapping is implemented.
- Load/store TLB misses now abort the faulting instruction instead of writing `0xFFFFFFFF` garbage.
- Delay-slot exceptions preserve branch EPC/BD behavior.
- Initial SOLO ASIC timer/RIO/scheduler experiments are instrumented.

The earlier type-2 adapter/no-carrier diagnostic rendered a printer-adapter warning, not normal built-in modem detection. The default now models no external adapter. Remote Power input wakes the real firmware, which renders its authentic `Connecting to WebTV` page with `Preparing to call` and a progress bar. This has been verified in the live guest framebuffer; native-window presentation confirmation remains pending. The optional TCP modem bridge forwards serial traffic to an external peer but does not implement PPP itself.

The SDL2 viewer opens while firmware runs. F1 sends remote Power On through the SOLO IR receiver; key repeats are ignored. Set `WEBTV_VIDEO=0` to disable the viewer. It loads `SDL2.dll` from `SDL2_DLL`, then the local `.tools` package, then the system path. The presentation path uses YUY2, based on the firmware-programmed display buffer.

Set `WEBTV_MODEM_TCP` to a TCP serial-modem peer such as `127.0.0.1:57388` to forward post-download UART bytes and receive peer data. The built-in modem model covers the boot download/carrier bring-up only; it does not claim to be a PPP/WTVP peer. `WEBTV_SYNTHETIC_PPP=1` enables an experimental LCP probe and is excluded from normal verification. Historical RAM-string shortcuts are opt-in with `WEBTV_COMPAT_SHIMS=1`; `WEBTV_WTV_PORT_RESPONSE=none` is equivalent to the default unmapped adapter port, and `type2-zero` retains the old printer-adapter diagnostic. `WEBTV_POWER_AT_STEP` schedules one hardware Power On input for headless verification, without changing guest RAM or event lists.

## Repository Layout

- `main.go` - boot harness and current trace/debug run loop.
- `cpu.go` - R4640/MIPS III interpreter and COP0/TLB handling.
- `bus.go` - physical memory/bus mapping.
- `asic.go` - SOLO ASIC register model and interrupt/timer stubs.
- `ide.go` - IDE disk image access.
- `*_test.go` - focused CPU/bus/ASIC/IDE regression tests.
- `roms/` - local-only firmware inputs. See `roms/README.md`.

## Local Files Not Committed

Firmware, ROM images, generated traces, framebuffer dumps, preview images, and built executables are intentionally ignored.

Expected local inputs include:

- `roms/bootrom.brom`
- `roms/ptnvram.iic` if available
- a WebTV Plus disk/flash image, passed on the command line

## Run And Test

This repo is plain Go and requires Go 1.21 or later.

```powershell
go test ./...
```

Run with a local firmware image:

```powershell
go run . "firmware.img"
```

## Notes

This is preservation/debug work. Do not commit proprietary ROMs, disk images, generated memory dumps, or local preview artifacts.
