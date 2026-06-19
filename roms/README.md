# Local ROM Inputs

Put local firmware inputs here. These files are intentionally not committed.

Expected names used by the current harness:

- `bootrom.brom` - WebTV Plus boot ROM, loaded at physical `0x1FC00000` and executed via `0xBFC00000`.
- `ptnvram.iic` - optional I2C EEPROM/NVRAM image.

Disk/flash `.img` files are passed to the emulator on the command line and should also remain local-only.