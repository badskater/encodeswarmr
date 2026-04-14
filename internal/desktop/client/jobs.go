package client

import (
	"context"
	"fmt"
	"strconv"
)

// ListJobs returns all jobs, optionally filtered by status and/or search term.
func (c *Client) ListJobs(ctx context.Context, status, search string) ([]Job, error) {
	q := buildQuery(map[string]string{"status": status, "search": search})
	var jobs []Job
	if err := c.request(ctx, "GET", "/jobs"+q, nil, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// ListJobsPaged returns a paginated page of jobs.
func (c *Client) ListJobsPaged(ctx context.Context, status, search, cursor string, pageSize int) (*Collection[Job], error) {
	ps := ""
	if pageSize > 0 {
		ps = strconv.Itoa(pageSize)
	}
	q := buildQuery(map[string]string{
		"status":    status,
		"search":    search,
		"cursor":    cursor,
		"page_size": ps,
	})
	return requestCollection[Job](c, ctx, "/jobs"+q)
}

// GetJob returns a job along with its tasks.
func (c *Client) GetJob(ctx context.Context, id string) (*JobDetail, error) {
	var detail JobDetail
	if err := c.request(ctx, "GET", "/jobs/"+id, nil, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// CreateJob submits a new encoding job.
func (c *Client) CreateJob(ctx context.Context, req *CreateJobRequest) (*Job, error) {
	var job Job
	if err := c.request(ctx, "POST", "/jobs", req, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// CancelJob cancels a running or queued job.
func (c *Client) CancelJob(ctx context.Context, id string) error {
	return c.request(ctx, "POST", "/jobs/"+id+"/cancel", nil, nil)
}

// RetryJob re-queues a failed job.
func (c *Client) RetryJob(ctx context.Context, id string) error {
	return c.request(ctx, "POST", "/jobs/"+id+"/retry", nil, nil)
}

// CreateJobChain creates a sequenced chain of jobs for a single source.
func (c *Client) CreateJobChain(ctx context.Context, req *CreateJobChainRequest) (*JobChainResponse, error) {
	var chain JobChainResponse
	if err := c.request(ctx, "POST", "/job-chains", req, &chain); err != nil {
		return nil, err
	}
	return &chain, nil
}

// GetJobChain returns all jobs belonging to the given chain group.
func (c *Client) GetJobChain(ctx context.Context, chainGroup string) (*JobChainResponse, error) {
	var chain JobChainResponse
	if err := c.request(ctx, "GET", "/job-chains/"+chainGroup, nil, &chain); err != nil {
		return nil, err
	}
	return &chain, nil
}

// BatchImportSources imports multiple source files matching a path pattern.
func (c *Client) BatchImportSources(ctx context.Context, req *BatchImportRequest) (*BatchImportResponse, error) {
	var result BatchImportResponse
	if err := c.request(ctx, "POST", "/sources/batch-import", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetJobComparison returns source-vs-output quality metrics for a completed job.
func (c *Client) GetJobComparison(ctx context.Context, jobID string) (*ComparisonResponse, error) {
	var cmp ComparisonResponse
	if err := c.request(ctx, "GET", "/jobs/"+jobID+"/comparison", nil, &cmp); err != nil {
		return nil, err
	}
	return &cmp, nil
}

// UpdateJobPriority changes the scheduling priority of a queued job.
func (c *Client) UpdateJobPriority(ctx context.Context, id string, priority int) error {
	body := map[string]int{"priority": priority}
	return c.request(ctx, "PUT", "/jobs/"+id+"/priority", body, nil)
}

// ReorderJobs updates the queue order for the given job IDs.
func (c *Client) ReorderJobs(ctx context.Context, jobIDs []string) error {
	body := map[string][]string{"job_ids": jobIDs}
	return c.request(ctx, "POST", "/jobs/reorder", body, nil)
}

// ListPendingJobsPaged returns a page of queued jobs (up to 200 per page).
func (c *Client) ListPendingJobsPaged(ctx context.Context, cursor string) (*Collection[Job], error) {
	q := buildQuery(map[string]string{
		"status":    "queued",
		"page_size": "200",
		"cursor":    cursor,
	})
	return requestCollection[Job](c, ctx, "/jobs"+q)
}

// ListArchivedJobs returns a paginated list of archived jobs.
func (c *Client) ListArchivedJobs(ctx context.Context, status, cursor string, pageSize int) (*Collection[Job], error) {
	ps := ""
	if pageSize > 0 {
		ps = strconv.Itoa(pageSize)
	}
	q := buildQuery(map[string]string{
		"status":    status,
		"cursor":    cursor,
		"page_size": ps,
	})
	return requestCollection[Job](c, ctx, "/archive/jobs"+q)
}

// JobExportURL returns the URL for the job export endpoint (CSV or JSON download).
func (c *Client) JobExportURL(format, status, from, to string) string {
	q := buildQuery(map[string]string{
		"format": format,
		"status": status,
		"from":   from,
		"to":     to,
	})
	return fmt.Sprintf("%s/api/v1/jobs/export%s", c.baseURL, q)
}

// ArchivedJobExportURL returns the URL for the archived job export endpoint.
func (c *Client) ArchivedJobExportURL(format, status, from, to string) string {
	q := buildQuery(map[string]string{
		"format": format,
		"status": status,
		"from":   from,
		"to":     to,
	})
	return fmt.Sprintf("%s/api/v1/archive/jobs/export%s", c.baseURL, q)
}
