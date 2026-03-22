package api

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ComparisonResponse is the response payload for GET /api/v1/jobs/{id}/comparison.
// It aggregates source and output metrics across all completed encode tasks.
type ComparisonResponse struct {
	Source            comparisonSide `json:"source"`
	Output            comparisonSide `json:"output"`
	CompressionRatio  float64        `json:"compression_ratio"`
	SizeReductionPct  float64        `json:"size_reduction_pct"`
	VMafScore         *float64       `json:"vmaf_score,omitempty"`
	PSNR              *float64       `json:"psnr,omitempty"`
	SSIM              *float64       `json:"ssim,omitempty"`
}

type comparisonSide struct {
	DurationSec float64 `json:"duration_sec"`
	FileSizeMB  float64 `json:"file_size_mb"`
	Codec       string  `json:"codec,omitempty"`
	Resolution  string  `json:"resolution,omitempty"`
}

// handleGetJobComparison returns source-vs-output metrics for a completed job.
//
// GET /api/v1/jobs/{id}/comparison
func (s *Server) handleGetJobComparison(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing job id")
		return
	}

	job, err := s.store.GetJobByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "job not found")
			return
		}
		s.logger.Error("comparison: get job", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	tasks, err := s.store.ListTasksByJob(r.Context(), id)
	if err != nil {
		s.logger.Error("comparison: list tasks", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Aggregate quality and size metrics from completed encode tasks.
	var totalOutputSizeBytes int64
	var totalDurationSec int64
	var vmafSum, psnrSum, ssimSum float64
	var vmafCount, psnrCount, ssimCount int

	for _, t := range tasks {
		if t.TaskType == db.TaskTypeConcat || t.Status != "completed" {
			continue
		}
		if t.OutputSize != nil {
			totalOutputSizeBytes += *t.OutputSize
		}
		if t.DurationSec != nil {
			totalDurationSec += *t.DurationSec
		}
		if t.VMafScore != nil {
			vmafSum += *t.VMafScore
			vmafCount++
		}
		if t.PSNR != nil {
			psnrSum += *t.PSNR
			psnrCount++
		}
		if t.SSIM != nil {
			ssimSum += *t.SSIM
			ssimCount++
		}
	}

	// Retrieve source metadata from analysis results.
	analysisResults, _ := s.store.ListAnalysisResults(r.Context(), job.SourceID)
	var srcCodec, srcResolution string
	var srcDurationSec float64
	var srcSizeBytes int64

	for _, ar := range analysisResults {
		if len(ar.Summary) == 0 {
			continue
		}
		var sum map[string]any
		if jsonErr := json.Unmarshal(ar.Summary, &sum); jsonErr != nil {
			continue
		}
		if c, ok := sum["codec"].(string); ok && c != "" && srcCodec == "" {
			srcCodec = c
		}
		if w, ok := sum["width"].(float64); ok {
			if h, ok2 := sum["height"].(float64); ok2 && w > 0 && h > 0 && srcResolution == "" {
				srcResolution = formatResolution(int(w), int(h))
			}
		}
		if d, ok := sum["duration_sec"].(float64); ok && d > 0 && srcDurationSec == 0 {
			srcDurationSec = d
		}
	}

	// Fall back to source size_bytes from the source record.
	src, srcErr := s.store.GetSourceByID(r.Context(), job.SourceID)
	if srcErr == nil {
		srcSizeBytes = src.SizeBytes
	}

	if srcDurationSec == 0 && totalDurationSec > 0 {
		srcDurationSec = float64(totalDurationSec)
	}

	outputFileSizeMB := float64(totalOutputSizeBytes) / 1_000_000
	sourceFileSizeMB := float64(srcSizeBytes) / 1_000_000

	var compressionRatio, sizeReductionPct float64
	if outputFileSizeMB > 0 && sourceFileSizeMB > 0 {
		compressionRatio = math.Round(sourceFileSizeMB/outputFileSizeMB*100) / 100
		sizeReductionPct = math.Round((1-outputFileSizeMB/sourceFileSizeMB)*10000) / 100
	}

	resp := ComparisonResponse{
		Source: comparisonSide{
			DurationSec: srcDurationSec,
			FileSizeMB:  math.Round(sourceFileSizeMB*100) / 100,
			Codec:       srcCodec,
			Resolution:  srcResolution,
		},
		Output: comparisonSide{
			DurationSec: float64(totalDurationSec),
			FileSizeMB:  math.Round(outputFileSizeMB*100) / 100,
		},
		CompressionRatio: compressionRatio,
		SizeReductionPct: sizeReductionPct,
	}

	if vmafCount > 0 {
		v := math.Round(vmafSum/float64(vmafCount)*100) / 100
		resp.VMafScore = &v
	}
	if psnrCount > 0 {
		v := math.Round(psnrSum/float64(psnrCount)*100) / 100
		resp.PSNR = &v
	}
	if ssimCount > 0 {
		v := math.Round(ssimSum/float64(ssimCount)*10000) / 10000
		resp.SSIM = &v
	}

	writeJSON(w, r, http.StatusOK, resp)
}

// formatResolution formats pixel dimensions as "WxH".
func formatResolution(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	return intToStr(w) + "x" + intToStr(h)
}

// intToStr converts an int to its decimal string representation without
// importing strconv (it is already imported elsewhere in the package).
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
