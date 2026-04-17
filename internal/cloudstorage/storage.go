// Package cloudstorage provides a unified interface for downloading and
// uploading files from/to S3, Google Cloud Storage, and Azure Blob Storage.
// Callers use NewStore to obtain a Store for a given URI scheme; the returned
// implementation reads credentials from environment variables or ambient
// identity (instance roles, workload identity, managed identity).
//
// Retry behaviour
//
// Each adapter uses the SDK's built-in retry machinery so that transient
// network errors, 429 Too Many Requests, and 5xx responses are retried with
// exponential back-off:
//
//   - S3:    AWS SDK v2 standard retry mode (RetryModeStandard).
//   - Azure: Azure SDK built-in retry policy (policy.RetryOptions).
//   - GCS:   client.SetRetry with gax.Backoff + WithErrorFuncWithContext for
//     structured per-attempt logging.
//
// The retry timing is controlled by RetryConfig; pass a non-zero value to
// NewStoreWithConfig, or call NewStore to use the defaults.
package cloudstorage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	// AWS SDK v2
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	// Google Cloud Storage
	gcs "cloud.google.com/go/storage"
	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"

	// Azure Blob
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// RetryConfig controls the exponential back-off applied to transient cloud
// storage errors.  A zero-value RetryConfig is valid and uses the defaults
// listed in DefaultRetryConfig.
type RetryConfig struct {
	// MaxElapsed is the maximum total time to spend retrying one operation.
	// Default: 5 minutes.
	MaxElapsed time.Duration
	// InitialInterval is the delay before the first retry.
	// Default: 500 ms.
	InitialInterval time.Duration
	// MaxInterval is the cap on the per-attempt delay.
	// Default: 30 s.
	MaxInterval time.Duration
	// Multiplier is the growth factor applied after each attempt.
	// Default: 2.0.
	Multiplier float64
}

// DefaultRetryConfig is the out-of-the-box retry policy.
var DefaultRetryConfig = RetryConfig{
	MaxElapsed:      5 * time.Minute,
	InitialInterval: 500 * time.Millisecond,
	MaxInterval:     30 * time.Second,
	Multiplier:      2.0,
}

func (rc RetryConfig) withDefaults() RetryConfig {
	if rc.MaxElapsed == 0 {
		rc.MaxElapsed = DefaultRetryConfig.MaxElapsed
	}
	if rc.InitialInterval == 0 {
		rc.InitialInterval = DefaultRetryConfig.InitialInterval
	}
	if rc.MaxInterval == 0 {
		rc.MaxInterval = DefaultRetryConfig.MaxInterval
	}
	if rc.Multiplier == 0 {
		rc.Multiplier = DefaultRetryConfig.Multiplier
	}
	return rc
}

// s3MaxAttempts converts a MaxElapsed duration into an approximate upper bound
// on the number of SDK retry attempts.  The AWS SDK standard mode uses its own
// back-off schedule; we clamp to a reasonable maximum so it still respects the
// configured ceiling.  We estimate ~8 attempts fit comfortably within 5 min at
// the default timing; scale linearly from there.
func s3MaxAttempts(cfg RetryConfig) int {
	// Baseline: 8 attempts ≈ 5 min with default timing.
	base := int(cfg.MaxElapsed / (5 * time.Minute) * 8)
	if base < 3 {
		base = 3
	}
	if base > 20 {
		base = 20
	}
	return base
}

// Store is the cloud storage abstraction used by the agent to fetch source
// files and (optionally) upload encoded output.
type Store interface {
	// Download fetches the object at uri and writes it to destPath.
	Download(ctx context.Context, uri, destPath string) error
	// Upload sends the file at srcPath to the object address uri.
	Upload(ctx context.Context, srcPath, uri string) error
	// Exists reports whether the object at uri exists.
	Exists(ctx context.Context, uri string) (bool, error)
}

// NewStore returns a Store appropriate for the scheme of uri using the default
// retry policy.  It is equivalent to NewStoreWithConfig(uri, RetryConfig{}).
// Supported schemes: s3://, gs://, az://.
func NewStore(uri string) (Store, error) {
	return NewStoreWithConfig(uri, RetryConfig{})
}

// NewStoreWithConfig returns a Store appropriate for the scheme of uri,
// configured with the supplied RetryConfig.  Zero fields in cfg fall back to
// DefaultRetryConfig values.
func NewStoreWithConfig(uri string, cfg RetryConfig) (Store, error) {
	cfg = cfg.withDefaults()
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: parse uri %q: %w", uri, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "s3":
		return newS3Store(u, cfg)
	case "gs":
		return newGCSStore(u, cfg)
	case "az":
		return newAzureStore(u, cfg)
	default:
		return nil, fmt.Errorf("cloudstorage: unsupported scheme %q (want s3://, gs://, or az://)", u.Scheme)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseS3URI splits an s3://bucket/key URI into (bucket, key).
func parseS3URI(uri string) (bucket, key string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("parse s3 uri: %w", err)
	}
	bucket = u.Host
	key = strings.TrimPrefix(u.Path, "/")
	if bucket == "" || key == "" {
		return "", "", fmt.Errorf("s3 uri %q must be s3://bucket/key", uri)
	}
	return bucket, key, nil
}

// parseGCSURI splits a gs://bucket/object URI into (bucket, object).
func parseGCSURI(uri string) (bucket, object string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("parse gcs uri: %w", err)
	}
	bucket = u.Host
	object = strings.TrimPrefix(u.Path, "/")
	if bucket == "" || object == "" {
		return "", "", fmt.Errorf("gcs uri %q must be gs://bucket/object", uri)
	}
	return bucket, object, nil
}

// parseAzureURI splits an az://container/blob URI into (container, blob).
func parseAzureURI(uri string) (container, blob string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("parse azure uri: %w", err)
	}
	container = u.Host
	blob = strings.TrimPrefix(u.Path, "/")
	if container == "" || blob == "" {
		return "", "", fmt.Errorf("azure uri %q must be az://container/blob", uri)
	}
	return container, blob, nil
}

// writeToFile creates destPath (and any required parent directories) and
// copies all bytes from r into it.
func writeToFile(destPath string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("mkdirall: %w", err)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create dest file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write dest file: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// S3
// ---------------------------------------------------------------------------

// loggingRetryer wraps retry.Standard and emits a structured slog record on
// every retry decision, matching the log shape used by the GCS adapter so
// that ops dashboards can unify them across providers.
//
// Fields logged on each retry:
//
//	attempt    – 1-based attempt number that just failed
//	operation  – caller-supplied label (e.g. "GetObject")
//	bucket     – S3 bucket name
//	key        – S3 object key
//	error      – the error that triggered the retry
//	next_delay – delay the SDK will apply before the next attempt
//
// Warn is used for individual retries; Error is logged when the final attempt
// fails (i.e. IsErrorRetryable returns false after exhausting maxAttempts).
type loggingRetryer struct {
	*retry.Standard
	logger    *slog.Logger
	operation string
	bucket    string
	key       string
}

// IsErrorRetryable delegates to the embedded Standard retryer.  Logging is
// done by the retry middleware via RetryDelay; we do not log here to avoid
// double-logging.
func (r *loggingRetryer) IsErrorRetryable(err error) bool {
	return r.Standard.IsErrorRetryable(err)
}

// RetryDelay delegates to Standard and logs the upcoming retry at Warn level.
// attempt is 1-based (the attempt that just failed) so attempt 1 means the
// first attempt failed and we are about to retry.
func (r *loggingRetryer) RetryDelay(attempt int, err error) (time.Duration, error) {
	d, delayErr := r.Standard.RetryDelay(attempt, err)
	if delayErr == nil {
		r.logger.Warn("cloudstorage s3 retry",
			"attempt", attempt,
			"operation", r.operation,
			"bucket", r.bucket,
			"key", r.key,
			"error", err,
			"next_delay", d,
		)
	} else {
		// RetryDelay returning an error means we have exhausted the budget.
		r.logger.Error("cloudstorage s3 retry exhausted",
			"attempt", attempt,
			"operation", r.operation,
			"bucket", r.bucket,
			"key", r.key,
			"error", err,
		)
	}
	return d, delayErr
}

// withContext returns a shallow copy of the loggingRetryer with the
// operation/bucket/key fields set.  This is used by the s3Store methods to
// stamp per-call context onto the retryer before passing it to the SDK.
func (r *loggingRetryer) withContext(operation, bucket, key string) *loggingRetryer {
	cp := *r
	cp.operation = operation
	cp.bucket = bucket
	cp.key = key
	return &cp
}

// s3Store uses the AWS SDK v2 standard retry mode which retries on 429,
// 5xx, and transient network errors with exponential back-off and jitter.
// A loggingRetryer wraps the standard retryer to emit structured slog records
// on each retry attempt, matching the GCS adapter's log shape.
type s3Store struct {
	client  *s3.Client
	retryer *loggingRetryer
}

func newS3Store(_ *url.URL, cfg RetryConfig) (*s3Store, error) {
	logger := slog.Default()
	lr := &loggingRetryer{
		Standard: retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = s3MaxAttempts(cfg)
		}),
		logger: logger,
	}

	awsCfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRetryer(func() awssdk.Retryer { return lr }),
	)
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: load aws config: %w", err)
	}
	return &s3Store{client: s3.NewFromConfig(awsCfg), retryer: lr}, nil
}

// s3Retryer returns a per-call loggingRetryer stamped with operation/bucket/key
// so that retry log records carry the full object address.  If the store was
// constructed without a loggingRetryer (e.g. in tests using newFakeS3Client)
// the returned option is a no-op so the SDK's pre-configured retryer is used.
func (st *s3Store) s3Retryer(operation, bucket, key string) func(*s3.Options) {
	if st.retryer == nil {
		return func(*s3.Options) {} // no-op; SDK uses its own retryer
	}
	lr := st.retryer.withContext(operation, bucket, key)
	return func(o *s3.Options) {
		o.Retryer = lr
	}
}

func (st *s3Store) Download(ctx context.Context, uri, destPath string) error {
	bucket, key, err := parseS3URI(uri)
	if err != nil {
		return fmt.Errorf("cloudstorage s3 download: %w", err)
	}
	resp, err := st.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, st.s3Retryer("GetObject", bucket, key))
	if err != nil {
		return fmt.Errorf("cloudstorage s3 get object %q: %w", uri, err)
	}
	defer resp.Body.Close()
	return writeToFile(destPath, resp.Body)
}

func (st *s3Store) Upload(ctx context.Context, srcPath, uri string) error {
	bucket, key, err := parseS3URI(uri)
	if err != nil {
		return fmt.Errorf("cloudstorage s3 upload: %w", err)
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("cloudstorage s3 open source: %w", err)
	}
	defer f.Close()
	_, err = st.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   f,
	}, st.s3Retryer("PutObject", bucket, key))
	if err != nil {
		return fmt.Errorf("cloudstorage s3 put object %q: %w", uri, err)
	}
	return nil
}

func (st *s3Store) Exists(ctx context.Context, uri string) (bool, error) {
	bucket, key, err := parseS3URI(uri)
	if err != nil {
		return false, fmt.Errorf("cloudstorage s3 exists: %w", err)
	}
	_, err = st.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, st.s3Retryer("HeadObject", bucket, key))
	if err != nil {
		// The SDK returns a NoSuchKey / 404 error; treat as not-found.
		return false, nil
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Google Cloud Storage
// ---------------------------------------------------------------------------

// gcsStore configures the GCS client with SDK-native retry via SetRetry.
// WithErrorFuncWithContext is used to emit a structured slog record on each
// retry attempt so operators can correlate transient failures without enabling
// verbose SDK-level debug logging.
type gcsStore struct {
	client *gcs.Client
}

func newGCSStore(_ *url.URL, cfg RetryConfig) (*gcsStore, error) {
	credsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var opts []option.ClientOption
	if credsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credsFile))
	}
	client, err := gcs.NewClient(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: create gcs client: %w", err)
	}

	client.SetRetry(
		gcs.WithBackoff(gax.Backoff{
			Initial:    cfg.InitialInterval,
			Max:        cfg.MaxInterval,
			Multiplier: cfg.Multiplier,
		}),
		gcs.WithMaxRetryDuration(cfg.MaxElapsed),
		// Retry all operations (not just idempotent ones) — uploads to GCS are
		// resumable so retrying is safe even for non-idempotent calls.
		gcs.WithPolicy(gcs.RetryAlways),
		// Per-attempt structured logging.  retryCtx may be nil for some
		// internal SDK paths so we guard against it.
		gcs.WithErrorFuncWithContext(func(err error, retryCtx *gcs.RetryContext) bool {
			retry := gcs.ShouldRetry(err)
			if retry && retryCtx != nil && retryCtx.Attempt > 1 {
				slog.Warn("cloudstorage gcs retry",
					"attempt", retryCtx.Attempt,
					"operation", retryCtx.Operation,
					"bucket", retryCtx.Bucket,
					"object", retryCtx.Object,
					"invocation_id", retryCtx.InvocationID,
					"error", err,
				)
			}
			return retry
		}),
	)

	return &gcsStore{client: client}, nil
}

func (st *gcsStore) Download(ctx context.Context, uri, destPath string) error {
	bucket, object, err := parseGCSURI(uri)
	if err != nil {
		return fmt.Errorf("cloudstorage gcs download: %w", err)
	}
	r, err := st.client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("cloudstorage gcs open object %q: %w", uri, err)
	}
	defer r.Close()
	return writeToFile(destPath, r)
}

func (st *gcsStore) Upload(ctx context.Context, srcPath, uri string) error {
	bucket, object, err := parseGCSURI(uri)
	if err != nil {
		return fmt.Errorf("cloudstorage gcs upload: %w", err)
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("cloudstorage gcs open source: %w", err)
	}
	defer f.Close()
	w := st.client.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err := io.Copy(w, f); err != nil {
		_ = w.Close()
		return fmt.Errorf("cloudstorage gcs write object %q: %w", uri, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("cloudstorage gcs close writer %q: %w", uri, err)
	}
	return nil
}

func (st *gcsStore) Exists(ctx context.Context, uri string) (bool, error) {
	bucket, object, err := parseGCSURI(uri)
	if err != nil {
		return false, fmt.Errorf("cloudstorage gcs exists: %w", err)
	}
	_, err = st.client.Bucket(bucket).Object(object).Attrs(ctx)
	if err == gcs.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cloudstorage gcs attrs %q: %w", uri, err)
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Azure Blob Storage
// ---------------------------------------------------------------------------

// azureStore uses the Azure SDK's built-in retry pipeline configured via
// policy.RetryOptions.  The SDK retries on 408, 429, 500, 502, 503, 504 and
// transient network errors with exponential back-off.
type azureStore struct {
	client *azblob.Client
}

func newAzureStore(_ *url.URL, cfg RetryConfig) (*azureStore, error) {
	account := os.Getenv("AZURE_STORAGE_ACCOUNT")
	key := os.Getenv("AZURE_STORAGE_KEY")
	if account == "" || key == "" {
		return nil, fmt.Errorf("cloudstorage: AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_KEY must be set")
	}
	cred, err := azblob.NewSharedKeyCredential(account, key)
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: create azure credential: %w", err)
	}
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", account)

	// Convert MaxElapsed into a MaxRetries count.  Azure's retry policy uses
	// an attempt count rather than a wall-clock deadline, so we approximate.
	azMaxRetries := int32(s3MaxAttempts(cfg))

	clientOpts := &azblob.ClientOptions{}
	clientOpts.Retry = policy.RetryOptions{
		MaxRetries:    azMaxRetries,
		RetryDelay:    cfg.InitialInterval,
		MaxRetryDelay: cfg.MaxInterval,
		// StatusCodes left nil → uses SDK defaults (408, 429, 500, 502, 503, 504).
	}

	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: create azure client: %w", err)
	}
	return &azureStore{client: client}, nil
}

func (st *azureStore) Download(ctx context.Context, uri, destPath string) error {
	container, blob, err := parseAzureURI(uri)
	if err != nil {
		return fmt.Errorf("cloudstorage azure download: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("cloudstorage azure mkdirall: %w", err)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("cloudstorage azure create dest: %w", err)
	}
	defer f.Close()
	_, err = st.client.DownloadFile(ctx, container, blob, f, nil)
	if err != nil {
		return fmt.Errorf("cloudstorage azure download blob %q: %w", uri, err)
	}
	return nil
}

func (st *azureStore) Upload(ctx context.Context, srcPath, uri string) error {
	container, blob, err := parseAzureURI(uri)
	if err != nil {
		return fmt.Errorf("cloudstorage azure upload: %w", err)
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("cloudstorage azure open source: %w", err)
	}
	defer f.Close()
	_, err = st.client.UploadFile(ctx, container, blob, f, nil)
	if err != nil {
		return fmt.Errorf("cloudstorage azure upload blob %q: %w", uri, err)
	}
	return nil
}

func (st *azureStore) Exists(ctx context.Context, uri string) (bool, error) {
	container, blob, err := parseAzureURI(uri)
	if err != nil {
		return false, fmt.Errorf("cloudstorage azure exists: %w", err)
	}
	pager := st.client.NewListBlobsFlatPager(container, &azblob.ListBlobsFlatOptions{
		Prefix: &blob,
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("cloudstorage azure list blobs: %w", err)
		}
		for _, item := range page.Segment.BlobItems {
			if item.Name != nil && *item.Name == blob {
				return true, nil
			}
		}
	}
	return false, nil
}
