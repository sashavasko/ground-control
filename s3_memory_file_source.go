// Package filesource provides reusable sources for S3 object transfers.
package filesource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	tmtypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
)

// ReadSeekerAt is the set of read operations required by consumers that may
// read sequentially, seek, or read independently at an absolute offset.
type ReadSeekerAt interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

// FileSource supplies independent readers over one immutable file.
//
// Every call to GetBody must return a reader with its own cursor, initially
// positioned at byte zero. Callers may therefore consume bodies concurrently.
type FileSource interface {
	GetBody() ReadSeekerAt
	Size() int64
}

// ObjectDownloader is implemented by *transfermanager.Client. Keeping the
// narrow interface makes S3MemoryFileSource straightforward to unit test.
type ObjectDownloader interface {
	DownloadObject(
		context.Context,
		*transfermanager.DownloadObjectInput,
		...func(*transfermanager.Options),
	) (*transfermanager.DownloadObjectOutput, error)
}

// S3MemoryFileSource downloads an S3 object once and retains its contents in
// memory. GetBody returns a new reader over the cached bytes on every call.
//
// The source is immutable after construction. GetBody and Size are safe for
// concurrent use.
type S3MemoryFileSource struct {
	data []byte
}

// NewS3MemoryFileSource creates a transfer manager and downloads the complete
// object into memory.
//
// Use this constructor when the caller owns an AWS S3 API client and does not
// need to share a specially configured transfer-manager client.
func NewS3MemoryFileSource(
	ctx context.Context,
	s3Client transfermanager.S3APIClient,
	bucket string,
	key string,
) (*S3MemoryFileSource, error) {
	if s3Client == nil {
		return nil, errors.New("S3 client cannot be nil")
	}

	downloader := transfermanager.New(
		s3Client,
		func(options *transfermanager.Options) {
			// Range-based downloads do not depend on the part boundaries used
			// when the source object was originally uploaded.
			options.GetObjectType = tmtypes.GetObjectRanges
		},
	)

	return NewS3MemoryFileSourceWithDownloader(
		ctx,
		downloader,
		bucket,
		key,
	)
}

// NewS3MemoryFileSourceWithDownloader downloads the complete object using an
// existing transfer-manager client. This form is useful for dependency
// injection and for sharing transfer-manager configuration.
func NewS3MemoryFileSourceWithDownloader(
	ctx context.Context,
	downloader ObjectDownloader,
	bucket string,
	key string,
) (*S3MemoryFileSource, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}
	if downloader == nil {
		return nil, errors.New("transfer manager cannot be nil")
	}
	if bucket == "" {
		return nil, errors.New("S3 bucket cannot be empty")
	}
	if key == "" {
		return nil, errors.New("S3 object key cannot be empty")
	}

	buffer := tmtypes.NewWriteAtBuffer(nil)

	_, err := downloader.DownloadObject(
		ctx,
		&transfermanager.DownloadObjectInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			WriterAt: buffer,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"download S3 object %q/%q into memory: %w",
			bucket,
			key,
			err,
		)
	}

	// WriteAtBuffer is no longer modified after DownloadObject returns. Keep
	// its completed byte slice without making a second object-sized copy.
	return &S3MemoryFileSource{
		data: buffer.Bytes(),
	}, nil
}

// GetBody creates an independent reader over the cached object. bytes.Reader
// implements io.Reader, io.ReaderAt, and io.Seeker. Creating a reader does not
// copy the cached bytes.
func (source *S3MemoryFileSource) GetBody() ReadSeekerAt {
	return bytes.NewReader(source.data)
}

// Size returns the size of the cached object in bytes.
func (source *S3MemoryFileSource) Size() int64 {
	return int64(len(source.data))
}

var (
	_ FileSource   = (*S3MemoryFileSource)(nil)
	_ ReadSeekerAt = (*bytes.Reader)(nil)
)
