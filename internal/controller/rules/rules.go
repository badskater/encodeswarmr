// Package rules implements the encoding rules engine.
//
// Rules are evaluated when a job is created to suggest (not auto-apply) a
// template, audio codec, priority, and tags based on source properties.
// The user always confirms before any suggestion is applied.
package rules

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/badskater/encodeswarmr/internal/db"
)

// SourceProperties holds the analysed properties of a source used for rule
// evaluation.  Fields map to the Condition.Field values.
type SourceProperties struct {
	// Resolution is the video resolution string, e.g. "1920x1080", "3840x2160".
	Resolution string
	// HDRType is the HDR format: "hdr10", "hdr10+", "dolby_vision", "hlg", or "".
	HDRType string
	// Codec is the video codec string, e.g. "h264", "hevc", "av1".
	Codec string
	// FileSizeGB is the source file size in gigabytes.
	FileSizeGB float64
	// DurationMin is the source duration in minutes.
	DurationMin float64
}

// Engine evaluates encoding rules against source properties.
type Engine struct {
	store  db.Store
	logger *slog.Logger
}

// New creates an Engine backed by the given store.
func New(store db.Store, logger *slog.Logger) *Engine {
	return &Engine{store: store, logger: logger}
}

// Evaluate loads all enabled rules ordered by priority and returns the action
// of the first rule whose conditions all match props. Returns nil when no rule
// matches.
func (e *Engine) Evaluate(ctx context.Context, props SourceProperties) (*db.RuleAction, error) {
	rules, err := e.store.ListEncodingRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("rules: list rules: %w", err)
	}

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if matchesAll(r.Conditions, props) {
			action := r.Actions // copy
			e.logger.Debug("rules: rule matched",
				"rule_id", r.ID, "rule_name", r.Name,
				"suggest_template", action.SuggestTemplateID,
			)
			return &action, nil
		}
	}
	return nil, nil
}

// matchesAll returns true when every condition in the slice matches props.
func matchesAll(conditions []db.RuleCondition, props SourceProperties) bool {
	for _, c := range conditions {
		if !matchesOne(c, props) {
			return false
		}
	}
	return true
}

// matchesOne evaluates a single condition against source properties.
func matchesOne(c db.RuleCondition, props SourceProperties) bool {
	fieldVal := fieldValue(c.Field, props)

	switch c.Operator {
	case "eq":
		return strings.EqualFold(fieldVal, c.Value)
	case "neq":
		return !strings.EqualFold(fieldVal, c.Value)
	case "contains":
		return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(c.Value))
	case "in":
		for _, v := range strings.Split(c.Value, ",") {
			if strings.EqualFold(strings.TrimSpace(v), fieldVal) {
				return true
			}
		}
		return false
	case "gt", "lt", "gte", "lte":
		fv, err := strconv.ParseFloat(fieldVal, 64)
		if err != nil {
			return false
		}
		cv, err := strconv.ParseFloat(c.Value, 64)
		if err != nil {
			return false
		}
		switch c.Operator {
		case "gt":
			return fv > cv
		case "lt":
			return fv < cv
		case "gte":
			return fv >= cv
		case "lte":
			return fv <= cv
		}
	}
	return false
}

// fieldValue returns the string representation of the named property.
func fieldValue(field string, props SourceProperties) string {
	switch field {
	case "resolution":
		return props.Resolution
	case "hdr_type":
		return props.HDRType
	case "codec":
		return props.Codec
	case "file_size_gb":
		return strconv.FormatFloat(props.FileSizeGB, 'f', -1, 64)
	case "duration_min":
		return strconv.FormatFloat(props.DurationMin, 'f', -1, 64)
	default:
		return ""
	}
}
