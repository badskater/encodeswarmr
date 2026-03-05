package service

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// gpuSample holds a single GPU utilisation reading.
type gpuSample struct {
	GPUPercent     float32
	EncoderPercent float32
	VRAMUsedMB     int64
}

// GPUInfo describes a detected GPU at startup.
type GPUInfo struct {
	Vendor string // "nvidia", "intel", "amd"
	Model  string
	VRAMМB int64
	NVENC  bool
	QSV    bool
	AMF    bool
}

// detectGPUs returns information about GPUs present on the host.
// All detection errors are silently swallowed — GPU detection is best-effort.
func detectGPUs() []GPUInfo {
	var gpus []GPUInfo
	gpus = append(gpus, detectNvidia()...)
	switch runtime.GOOS {
	case "windows":
		gpus = append(gpus, detectWMIGPUs()...)
	case "linux":
		gpus = append(gpus, detectLspciGPUs()...)
	}
	return gpus
}

// detectNvidia queries nvidia-smi for installed NVIDIA GPUs.
func detectNvidia() []GPUInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"nvidia-smi",
		"--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil
	}
	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}
		model := strings.TrimSpace(parts[0])
		vram, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		gpus = append(gpus, GPUInfo{
			Vendor: "nvidia",
			Model:  model,
			VRAMМB: vram,
			NVENC:  true,
		})
	}
	return gpus
}

// detectWMIGPUs queries WMI for Intel and AMD video controllers.
func detectWMIGPUs() []GPUInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"wmic", "path", "Win32_VideoController",
		"get", "Name,AdapterRAM", "/format:csv",
	).Output()
	if err != nil {
		return nil
	}
	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node") {
			continue
		}
		// CSV columns: Node,AdapterRAM,Name
		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 3 {
			continue
		}
		ram, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		name := strings.TrimSpace(parts[2])
		nameLower := strings.ToLower(name)

		var info GPUInfo
		info.Model = name
		info.VRAMМB = ram / 1024 / 1024

		switch {
		case strings.Contains(nameLower, "intel"):
			info.Vendor = "intel"
			info.QSV = true
		case strings.Contains(nameLower, "amd") || strings.Contains(nameLower, "radeon"):
			info.Vendor = "amd"
			info.AMF = true
		default:
			continue
		}
		gpus = append(gpus, info)
	}
	return gpus
}

// detectLspciGPUs queries lspci for Intel and AMD video controllers on Linux.
func detectLspciGPUs() []GPUInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// -mm produces machine-readable output; -v adds details.
	out, err := exec.CommandContext(ctx, "lspci", "-mm", "-v").Output()
	if err != nil {
		// lspci may not be installed; fall back to a simpler query.
		out, err = exec.CommandContext(ctx, "lspci").Output()
		if err != nil {
			return nil
		}
	}
	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga") &&
			!strings.Contains(lower, "display") &&
			!strings.Contains(lower, "3d controller") {
			continue
		}
		var info GPUInfo
		info.Model = strings.TrimSpace(line)
		// Trim the PCI address prefix (e.g. "00:02.0 VGA compatible controller: ")
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

// monitorGPU polls GPU utilisation at interval and sends samples to the
// returned channel. The channel is closed when ctx is cancelled.
// Only NVIDIA monitoring is implemented; other vendors emit zero samples.
func monitorGPU(ctx context.Context, vendor string, interval time.Duration) <-chan gpuSample {
	ch := make(chan gpuSample, 4)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sample := pollGPU(ctx, vendor)
				select {
				case ch <- sample:
				default:
				}
			}
		}
	}()
	return ch
}

// pollGPU performs a single GPU utilisation poll.
func pollGPU(ctx context.Context, vendor string) gpuSample {
	if vendor != "nvidia" {
		return gpuSample{}
	}
	pollCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(pollCtx,
		"nvidia-smi",
		"--query-gpu=utilization.gpu,utilization.encoder,memory.used",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return gpuSample{}
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	parts := strings.SplitN(line, ",", 3)
	if len(parts) < 3 {
		return gpuSample{}
	}
	gpuPct, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 32)
	encPct, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32)
	vramMB, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	return gpuSample{
		GPUPercent:     float32(gpuPct),
		EncoderPercent: float32(encPct),
		VRAMUsedMB:     vramMB,
	}
}
