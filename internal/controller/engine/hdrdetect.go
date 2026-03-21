package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"context"

	"github.com/badskater/distributed-encoder/internal/db"
)

// hdrDetectScriptBat is the Windows .bat script written to the task's script
// directory for hdr_detect jobs.  It uses ffprobe's flat output format to
// detect color metadata and optionally dovi_tool for the DV profile number.
// The final line printed to stdout is the sentinel the controller expects:
//
//	DE_HDR_RESULT={"hdr_type":"<type>","dv_profile":<n>}
//
// Tool paths are resolved from environment variables injected by the agent
// (FFPROBE_BIN, DOVI_TOOL_BIN) with PATH fallbacks.
// hdrDetectScriptBat is split using string concatenation at the two backtick
// pairs because Go raw string literals cannot contain backtick characters.
const hdrDetectScriptBat = `@echo off
setlocal enabledelayedexpansion

set "FFPROBE=%FFPROBE_BIN%"
if "!FFPROBE!"=="" set "FFPROBE=ffprobe"
set "DOVI_TOOL=%DOVI_TOOL_BIN%"
set "SOURCE=%SOURCE_PATH%"

set "HDR_TYPE="
set "DV_PROFILE=0"
set "HAS_DV="
set "HAS_HDR10P="
set "COLOR_TRANSFER="
set "CODEC_TAG="

rem Query color metadata and side-data types from the first video stream.
for /f "usebackq delims=" %%a in (` + "`" + `"%FFPROBE%" -v error -select_streams v:0 -show_entries stream=color_transfer,codec_tag_string -show_entries stream_side_data=type -of flat=s=_ "!SOURCE!" 2^>nul` + "`" + `) do (
    echo %%a | findstr /i "color_transfer" >nul 2>&1 && (
        for /f "tokens=2 delims==" %%v in ("%%a") do set "COLOR_TRANSFER=%%~v"
    )
    echo %%a | findstr /i "codec_tag_string" >nul 2>&1 && (
        for /f "tokens=2 delims==" %%v in ("%%a") do set "CODEC_TAG=%%~v"
    )
    echo %%a | findstr /i "DOVI_RPU_SIDE_DATA" >nul 2>&1 && set "HAS_DV=1"
    echo %%a | findstr /i "HDR_DYNAMIC_METADATA" >nul 2>&1 && set "HAS_HDR10P=1"
)

rem Classify HDR type from colour transfer characteristic.
if /i "!COLOR_TRANSFER!"=="smpte2084" set "HDR_TYPE=hdr10"
if /i "!COLOR_TRANSFER!"=="arib-std-b67" set "HDR_TYPE=hlg"

rem HDR10+ — dynamic metadata on top of HDR10.
if defined HAS_HDR10P if /i "!HDR_TYPE!"=="hdr10" set "HDR_TYPE=hdr10+"

rem Dolby Vision — detected via side-data or codec tag.
if defined HAS_DV (
    set "HDR_TYPE=dolby_vision"
) else (
    if /i "!CODEC_TAG!"=="dvh1" set "HDR_TYPE=dolby_vision"
    if /i "!CODEC_TAG!"=="dvhe" set "HDR_TYPE=dolby_vision"
)

rem Use dovi_tool for the DV profile number (optional).
if /i "!HDR_TYPE!"=="dolby_vision" (
    if not "!DOVI_TOOL!"=="" (
        for /f "usebackq delims=" %%d in (` + "`" + `"!DOVI_TOOL!" info -i "!SOURCE!" 2^>nul` + "`" + `) do (
            echo %%d | findstr /ri "\"profile\"" >nul 2>&1 && (
                for /f "tokens=2 delims=: " %%p in ("%%d") do (
                    set "DV_PROFILE=%%p"
                    set "DV_PROFILE=!DV_PROFILE:,=!"
                    goto :dv_done
                )
            )
        )
        :dv_done
    )
)

echo DE_HDR_RESULT={"hdr_type":"!HDR_TYPE!","dv_profile":!DV_PROFILE!}
`

// hdrDetectScriptSh is the POSIX sh script written for Linux/macOS agents.
const hdrDetectScriptSh = `#!/bin/sh
# HDR detection — outputs the sentinel line read by the controller:
#   DE_HDR_RESULT={"hdr_type":"<type>","dv_profile":<n>}
set -e

FFPROBE="${FFPROBE_BIN:-ffprobe}"
DOVI_TOOL="${DOVI_TOOL_BIN:-}"
SOURCE="${SOURCE_PATH}"

HDR_TYPE=""
DV_PROFILE=0

# Retrieve colour metadata and side-data types from the first video stream.
PROBE="$("${FFPROBE}" -v error -select_streams v:0 \
    -show_entries stream=color_transfer,codec_tag_string \
    -show_entries stream_side_data=type \
    -of flat=s=_ \
    "${SOURCE}" 2>/dev/null || true)"

COLOR_TRANSFER="$(printf '%s\n' "${PROBE}" | grep 'color_transfer' | cut -d= -f2 | tr -d '"' | head -1)"
CODEC_TAG="$(printf '%s\n' "${PROBE}" | grep 'codec_tag_string' | cut -d= -f2 | tr -d '"' | head -1)"
HAS_DV="$(printf '%s\n' "${PROBE}" | grep -c 'DOVI_RPU_SIDE_DATA' || true)"
HAS_HDR10P="$(printf '%s\n' "${PROBE}" | grep -c 'HDR_DYNAMIC_METADATA' || true)"

case "${COLOR_TRANSFER}" in
    smpte2084)    HDR_TYPE="hdr10" ;;
    arib-std-b67) HDR_TYPE="hlg"  ;;
esac

# HDR10+ — dynamic metadata layer on top of HDR10.
if [ "${HAS_HDR10P}" -gt 0 ] && [ "${HDR_TYPE}" = "hdr10" ]; then
    HDR_TYPE="hdr10+"
fi

# Dolby Vision — detected via side-data or codec tag.
if [ "${HAS_DV}" -gt 0 ] || \
   printf '%s\n' "${CODEC_TAG}" | grep -qiE '^(dvh1|dvhe)$'; then
    HDR_TYPE="dolby_vision"
    # Use dovi_tool for the specific profile number if available.
    if [ -n "${DOVI_TOOL}" ] && command -v "${DOVI_TOOL}" >/dev/null 2>&1; then
        DV_PROFILE_VAL="$( \
            "${DOVI_TOOL}" info -i "${SOURCE}" 2>/dev/null \
            | grep -iE '"profile"' \
            | grep -oE '[0-9]+' \
            | head -1 \
            || true)"
        if [ -n "${DV_PROFILE_VAL}" ]; then
            DV_PROFILE="${DV_PROFILE_VAL}"
        fi
    fi
fi

printf 'DE_HDR_RESULT={"hdr_type":"%s","dv_profile":%d}\n' "${HDR_TYPE}" "${DV_PROFILE}"
`

// HDRResultSentinel is the prefix the controller searches for in task stdout logs.
const HDRResultSentinel = "DE_HDR_RESULT="

// expandHDRDetectJob creates one task for the source and writes the built-in
// hdr_detect scripts (both .bat and .sh) to its script directory.  The agent
// executes the platform-appropriate script, which outputs a sentinel line read
// back by the controller to update sources.hdr_type and sources.dv_profile.
func (e *Engine) expandHDRDetectJob(ctx context.Context, job *db.Job) error {
	source, err := e.store.GetSourceByID(ctx, job.SourceID)
	if err != nil {
		return fmt.Errorf("engine: hdr_detect get source %s: %w", job.SourceID, err)
	}

	// Collect global variables and job ExtraVars to inject as task variables.
	vars, err := e.store.ListVariables(ctx, "")
	if err != nil {
		return fmt.Errorf("engine: hdr_detect load variables: %w", err)
	}

	variables := make(map[string]string, len(vars)+8)
	for _, v := range vars {
		variables[v.Name] = v.Value
	}
	for k, v := range job.EncodeConfig.ExtraVars {
		variables[k] = v
	}
	// SOURCE_PATH is always the authoritative source location.
	variables["SOURCE_PATH"] = source.UNCPath

	// failJob deletes any tasks already created for this job, then marks it
	// failed, preventing orphan tasks when expansion only partially succeeds.
	failJob := func(cause error) error {
		if delErr := e.store.DeleteTasksByJobID(ctx, job.ID); delErr != nil {
			e.logger.Error("engine: cleanup orphan tasks failed",
				"job_id", job.ID, "error", delErr)
		}
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return cause
	}

	task, err := e.store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: source.UNCPath,
		Variables:  variables,
	})
	if err != nil {
		return failJob(fmt.Errorf("engine: hdr_detect create task: %w", err))
	}

	// Write built-in scripts to the task's script directory.
	dir := filepath.Join(e.gen.baseDir, job.ID, "0000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return failJob(fmt.Errorf("engine: hdr_detect mkdir: %w", err))
	}

	if err := os.WriteFile(filepath.Join(dir, "run.bat"), []byte(hdrDetectScriptBat), 0o644); err != nil {
		return failJob(fmt.Errorf("engine: hdr_detect write bat: %w", err))
	}
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte(hdrDetectScriptSh), 0o755); err != nil {
		return failJob(fmt.Errorf("engine: hdr_detect write sh: %w", err))
	}

	if err := e.store.SetTaskScriptDir(ctx, task.ID, dir); err != nil {
		return failJob(fmt.Errorf("engine: hdr_detect set script dir: %w", err))
	}

	if err := e.store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		return fmt.Errorf("engine: hdr_detect update task counts: %w", err)
	}
	if err := e.store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		return fmt.Errorf("engine: hdr_detect update job status: %w", err)
	}

	e.logger.Info("hdr_detect job expanded",
		"job_id", job.ID,
		"source_id", source.ID,
		"source", source.UNCPath,
	)
	return nil
}
