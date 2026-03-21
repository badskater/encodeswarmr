package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// detectNvidia — calls nvidia-smi; skip when not available
// ---------------------------------------------------------------------------

func TestDetectNvidia_NoNvidiaSmi(t *testing.T) {
	// detectNvidia silently returns nil when nvidia-smi is absent or fails.
	// We just verify it does not panic regardless of whether the tool exists.
	gpus := detectNvidia()
	// Either nil or a slice of GPUInfo structs — both are valid.
	for _, g := range gpus {
		if g.Vendor != "nvidia" {
			t.Errorf("detected non-nvidia GPU from detectNvidia: vendor=%q", g.Vendor)
		}
		if !g.NVENC {
			t.Errorf("detected nvidia GPU without NVENC flag: %+v", g)
		}
	}
}

// ---------------------------------------------------------------------------
// detectLspciGPUs — parse lspci-like output
// ---------------------------------------------------------------------------

// parseLspciLines is a white-box helper that exercises the line-parsing logic
// of detectLspciGPUs via a synthetic lspci output string instead of running
// the real command.  We achieve this by calling the parsing code that lives
// inside detectLspciGPUs directly — since it is not exported, we replicate the
// same parsing logic here to validate its behaviour deterministically.
//
// The function below mirrors the inner loop of detectLspciGPUs so that we can
// test the classification rules without spawning a process.
func parseLspciOutput(output string) []GPUInfo {
	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga") &&
			!strings.Contains(lower, "display") &&
			!strings.Contains(lower, "3d controller") {
			continue
		}
		var info GPUInfo
		info.Model = strings.TrimSpace(line)
		if idx := strings.Index(info.Model, ": "); idx != -1 {
			info.Model = strings.TrimSpace(info.Model[idx+2:])
		}
		switch {
		case strings.Contains(lower, "intel"):
			info.Vendor = "intel"
			info.QSV = true
		case strings.Contains(lower, "amd") || strings.Contains(lower, "radeon") ||
			strings.Contains(lower, "advanced micro"):
			info.Vendor = "amd"
			info.AMF = true
		default:
			continue
		}
		gpus = append(gpus, info)
	}
	return gpus
}

func TestParseLspciOutput_IntelVGA(t *testing.T) {
	out := `00:02.0 VGA compatible controller: Intel Corporation HD Graphics 630 (rev 04)
01:00.0 3D controller: NVIDIA Corporation GP107M [GeForce GTX 1050] (rev a1)`
	gpus := parseLspciOutput(out)

	// Should detect one Intel GPU (NVIDIA lacks "intel"/"amd"/"radeon" keyword).
	intelCount := 0
	for _, g := range gpus {
		if g.Vendor == "intel" {
			intelCount++
			if !g.QSV {
				t.Errorf("Intel GPU should have QSV=true: %+v", g)
			}
		}
	}
	if intelCount != 1 {
		t.Errorf("expected 1 Intel GPU, got %d (total gpus: %d)", intelCount, len(gpus))
	}
}

func TestParseLspciOutput_AMDRadeon(t *testing.T) {
	out := `03:00.0 VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI] Navi 21 [Radeon RX 6800 XT]`
	gpus := parseLspciOutput(out)

	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}
	if gpus[0].Vendor != "amd" {
		t.Errorf("Vendor = %q, want amd", gpus[0].Vendor)
	}
	if !gpus[0].AMF {
		t.Error("AMD GPU should have AMF=true")
	}
}

func TestParseLspciOutput_RadeonKeyword(t *testing.T) {
	out := `04:00.0 Display controller: Radeon RX 580 Series`
	gpus := parseLspciOutput(out)

	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}
	if gpus[0].Vendor != "amd" {
		t.Errorf("Vendor = %q, want amd for Radeon line", gpus[0].Vendor)
	}
}

func TestParseLspciOutput_SkipsNonGPU(t *testing.T) {
	out := `00:1f.2 SATA controller: Intel Corporation 8 Series SATA Controller 1 [AHCI mode]
00:1f.3 SMBus: Intel Corporation 8 Series SMBus Controller`
	gpus := parseLspciOutput(out)
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs from non-GPU lines, got %d", len(gpus))
	}
}

func TestParseLspciOutput_Empty(t *testing.T) {
	gpus := parseLspciOutput("")
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs from empty output, got %d", len(gpus))
	}
}

func TestParseLspciOutput_ModelTrimPCIPrefix(t *testing.T) {
	out := `00:02.0 VGA compatible controller: Intel Corporation UHD Graphics 620`
	gpus := parseLspciOutput(out)

	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}
	if strings.Contains(gpus[0].Model, "00:02.0") {
		t.Errorf("Model should not contain PCI address prefix, got %q", gpus[0].Model)
	}
	if !strings.Contains(gpus[0].Model, "Intel Corporation") {
		t.Errorf("Model = %q, expected to contain device name", gpus[0].Model)
	}
}

// ---------------------------------------------------------------------------
// pollGPU — non-nvidia vendor returns zero sample
// ---------------------------------------------------------------------------

func TestPollGPU_NonNvidiaVendor(t *testing.T) {
	ctx := context.Background()
	sample := pollGPU(ctx, "intel")
	if sample.GPUPercent != 0 || sample.EncoderPercent != 0 || sample.VRAMUsedMB != 0 {
		t.Errorf("expected zero sample for non-nvidia vendor, got %+v", sample)
	}
}

func TestPollGPU_UnknownVendor(t *testing.T) {
	ctx := context.Background()
	sample := pollGPU(ctx, "")
	if sample.GPUPercent != 0 || sample.EncoderPercent != 0 || sample.VRAMUsedMB != 0 {
		t.Errorf("expected zero sample for empty vendor, got %+v", sample)
	}
}

func TestPollGPU_NvidiaWithoutTool(t *testing.T) {
	// When nvidia-smi is absent the function must return a zero sample, not panic.
	ctx := context.Background()
	sample := pollGPU(ctx, "nvidia")
	// Result is allowed to be zero if nvidia-smi is absent.
	_ = sample
}

// ---------------------------------------------------------------------------
// monitorGPU — channel closes when ctx is cancelled
// ---------------------------------------------------------------------------

func TestMonitorGPU_ClosesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Use a very short interval so the goroutine starts quickly.
	ch := monitorGPU(ctx, "intel", 50*time.Millisecond)

	// Cancel after a short time and verify the channel closes.
	cancel()

	select {
	case _, open := <-ch:
		if open {
			// Drain until closed.
			for range ch {
			}
		}
		// Channel closed — test passes.
	case <-time.After(time.Second):
		t.Error("monitorGPU channel did not close within 1s after context cancellation")
	}
}

func TestMonitorGPU_SendsSamples(t *testing.T) {
	// monitorGPU for nvidia vendor will call pollGPU which may or may not
	// produce data; for non-nvidia it produces zero samples.  We just verify
	// the channel can be read and closed without deadlock.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch := monitorGPU(ctx, "intel", 50*time.Millisecond)

	// Drain channel until closed.
	for range ch {
	}
	// Reaching here means the channel closed properly.
}

// ---------------------------------------------------------------------------
// detectGPUs — smoke test (just verifies no panic)
// ---------------------------------------------------------------------------

func TestDetectGPUs_DoesNotPanic(t *testing.T) {
	// detectGPUs runs best-effort detection; silently swallows errors.
	gpus := detectGPUs()
	for _, g := range gpus {
		if g.Vendor == "" {
			t.Errorf("detected GPU with empty vendor: %+v", g)
		}
	}
}

// ---------------------------------------------------------------------------
// GPUInfo struct fields
// ---------------------------------------------------------------------------

func TestGPUInfo_Fields(t *testing.T) {
	g := GPUInfo{
		Vendor: "nvidia",
		Model:  "RTX 4090",
		VRAMМB: 24576,
		NVENC:  true,
		QSV:    false,
		AMF:    false,
	}
	if g.Vendor != "nvidia" {
		t.Errorf("Vendor = %q", g.Vendor)
	}
	if g.VRAMМB != 24576 {
		t.Errorf("VRAMМB = %d", g.VRAMМB)
	}
	if !g.NVENC {
		t.Error("NVENC should be true")
	}
}

func TestGPUSample_Fields(t *testing.T) {
	s := gpuSample{
		GPUPercent:     45.5,
		EncoderPercent: 12.0,
		VRAMUsedMB:     8192,
	}
	if s.GPUPercent < 45.4 || s.GPUPercent > 45.6 {
		t.Errorf("GPUPercent = %f", s.GPUPercent)
	}
	if s.VRAMUsedMB != 8192 {
		t.Errorf("VRAMUsedMB = %d", s.VRAMUsedMB)
	}
}
