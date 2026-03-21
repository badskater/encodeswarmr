// Package cloudstorage provides a unified interface for downloading and
// uploading files from/to S3, Google Cloud Storage, and Azure Blob Storage.
// Callers use NewStore to obtain a Store for a given URI scheme; the returned
// implementation reads credentials from environment variables or ambient
// identity (instance roles, workload identity, managed identity).
package cloudstorage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	// AWS SDK v2
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	// Google Cloud Storage
	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/option"

	// Azure Blob
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

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

// NewStore returns a Store appropriate for the scheme of uri.
// Supported schemes: s3://, gs://, az://.
func NewStore(uri string) (Store, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: parse uri %q: %w", uri, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "s3":
		return newS3Store(u)
	case "gs":
		return newGCSStore(u)
	case "az":
		return newAzureStore(u)
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

type s3Store struct {
	client *s3.Client
}

func newS3Store(_ *url.URL) (*s3Store, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: load aws config: %w", err)
	}
	return &s3Store{client: s3.NewFromConfig(cfg)}, nil
}

func (st *s3Store) Download(ctx context.Context, uri, destPath string) error {
	bucket, key, err := parseS3URI(uri)
	if err != nil {
		return fmt.Errorf("cloudstorage s3 download: %w", err)
	}
	resp, err := st.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
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
	})
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
	})
	if err != nil {
		// The SDK returns a NoSuchKey / 404 error; treat as not-found.
		return false, nil
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Google Cloud Storage
// ---------------------------------------------------------------------------

type gcsStore struct {
	client *gcs.Client
}

func newGCSStore(_ *url.URL) (*gcsStore, error) {
	credsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var opts []option.ClientOption
	if credsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credsFile))
	}
	client, err := gcs.NewClient(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("cloudstorage: create gcs client: %w", err)
	}
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

type azureStore struct {
	client *azblob.Client
}

func newAzureStore(_ *url.URL) (*azureStore, error) {
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
	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
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
