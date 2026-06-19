package main

import "testing"

func TestIDECheckPowerModeReportsActive(t *testing.T) {
	ide := NewIDEController(nil)

	ide.execCommand(ATA_CMD_CHECK_POWER)

	if ide.Status != ATA_STAT_DRDY {
		t.Fatalf("Check Power Mode status=0x%02X, want DRDY", ide.Status)
	}
	if ide.Error != 0 {
		t.Fatalf("Check Power Mode error=0x%02X, want 0", ide.Error)
	}
	if ide.SectorCount != 0xFF {
		t.Fatalf("Check Power Mode sector count=0x%02X, want 0xFF active/idle", ide.SectorCount)
	}
}

func TestIDENewIdleCommandCompletesReady(t *testing.T) {
	ide := NewIDEController(nil)

	ide.execCommand(ATA_CMD_IDLE_NEW)

	if ide.Status != ATA_STAT_DRDY {
		t.Fatalf("Idle status=0x%02X, want DRDY", ide.Status)
	}
	if ide.Error != 0 {
		t.Fatalf("Idle error=0x%02X, want 0", ide.Error)
	}
}
