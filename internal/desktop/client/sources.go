package client

import (
	"context"
	"strconv"
)

// ListSources returns sources, optionally filtered by state and paginated.
func (c *Client) ListSources(ctx context.Context, state, cursor string, pageSize int) ([]Source, error) {
	ps := ""
	if pageSize > 0 {
		ps = strconv.Itoa(pageSize)
	}
	q := buildQuery(map[string]string{
		"state":     state,
		"cursor":    cursor,
		"page_size": ps,
	})
	var sources []Source
	if err := c.request(ctx, "GET", "/sources"+q, nil, &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// ListSourcesPaged returns a paginated collection of sources.
func (c *Client) ListSourcesPaged(ctx context.Context, state, cursor string, pageSize int) (*Collection[Source], error) {
	ps := ""
	if pageSize > 0 {
		ps = strconv.Itoa(pageSize)
	}
	q := buildQuery(map[string]string{
		"state":     state,
		"cursor":    cursor,
		"page_size": ps,
	})
	return requestCollection[Source](c, ctx, "/sources"+q)
}

// CreateSource registers a new media source.
func (c *Client) CreateSource(ctx context.Context, path, name, cloudURI string) (*Source, error) {
	body := map[string]string{}
	if path != "" {
		body["path"] = path
	}
	if name != "" {
		body["name"] = name
	}
	if cloudURI != "" {
		body["cloud_uri"] = cloudURI
	}
	var src Source
	if err := c.request(ctx, "POST", "/sources", body, &src); err != nil {
		return nil, err
	}
	return &src, nil
}

// GetSource returns a single source by ID.
func (c *Client) GetSource(ctx context.Context, id string) (*Source, error) {
	var src Source
	if err := c.request(ctx, "GET", "/sources/"+id, nil, &src); err != nil {
		return nil, err
	}
	return &src, nil
}

// AnalyzeSource triggers a VMAF/scene detection analysis job for a source.
func (c *Client) AnalyzeSource(ctx context.Context, id string) (*Job, error) {
	var job Job
	if err := c.request(ctx, "POST", "/sources/"+id+"/analyze", nil, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// HDRDetectSource triggers HDR detection for a source.
func (c *Client) HDRDetectSource(ctx context.Context, id string) (map[string]string, error) {
	var result map[string]string
	if err := c.request(ctx, "POST", "/sources/"+id+"/hdr-detect", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateSourceHDR patches the HDR type and Dolby Vision profile for a source.
func (c *Client) UpdateSourceHDR(ctx context.Context, id, hdrType string, dvProfile int) (*Source, error) {
	body := map[string]any{
		"hdr_type":   hdrType,
		"dv_profile": dvProfile,
	}
	var src Source
	if err := c.request(ctx, "PATCH", "/sources/"+id+"/hdr", body, &src); err != nil {
		return nil, err
	}
	return &src, nil
}

// DeleteSource removes a source record.
func (c *Client) DeleteSource(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/sources/"+id, nil, nil)
}

// GetAnalysis returns the primary analysis result for a source.
func (c *Client) GetAnalysis(ctx context.Context, sourceID string) (*AnalysisResult, error) {
	var result AnalysisResult
	if err := c.request(ctx, "GET", "/analysis/"+sourceID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListAnalysisResults returns all analysis results for a source.
func (c *Client) ListAnalysisResults(ctx context.Context, sourceID string) ([]AnalysisResult, error) {
	var results []AnalysisResult
	if err := c.request(ctx, "GET", "/analysis/"+sourceID+"/all", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetSourceScenes returns detected scene boundaries for a source.
func (c *Client) GetSourceScenes(ctx context.Context, sourceID string) (*SceneData, error) {
	var scenes SceneData
	if err := c.request(ctx, "GET", "/sources/"+sourceID+"/scenes", nil, &scenes); err != nil {
		return nil, err
	}
	return &scenes, nil
}

// SubtitlesResponse is the response from the subtitles endpoint.
type SubtitlesResponse struct {
	SourceID string          `json:"source_id"`
	Tracks   []SubtitleTrack `json:"tracks"`
}

// GetSourceSubtitles returns the subtitle tracks detected in a source file.
func (c *Client) GetSourceSubtitles(ctx context.Context, sourceID string) (*SubtitlesResponse, error) {
	var resp SubtitlesResponse
	if err := c.request(ctx, "GET", "/sources/"+sourceID+"/subtitles", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ThumbnailsResponse is the response from the thumbnails endpoint.
type ThumbnailsResponse struct {
	SourceID   string   `json:"source_id"`
	Thumbnails []string `json:"thumbnails"`
}

// GetSourceThumbnails returns the generated thumbnail URLs for a source.
func (c *Client) GetSourceThumbnails(ctx context.Context, sourceID string) (*ThumbnailsResponse, error) {
	var resp ThumbnailsResponse
	if err := c.request(ctx, "GET", "/sources/"+sourceID+"/thumbnails", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
