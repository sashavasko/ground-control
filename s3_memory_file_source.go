// Package filesource provides reusable sources for S3 object transfers.
package filesource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

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
	GetBody(context.Context) (ReadSeekerAt, error)
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

// S3MemoryFileSource lazily downloads an S3 object and retains its contents in
// memory. GetBody returns a new reader over the cached bytes on every call.
//
// The first GetBody call starts the download. Concurrent GetBody calls share
// that attempt instead of downloading the object more than once. A successful
// result is cached permanently. A failed attempt is not cached, allowing a
// later GetBody call to retry.
type S3MemoryFileSource struct {
	downloader ObjectDownloader
	bucket     string
	key        string

	mu       sync.Mutex
	data     []byte
	cached   bool
	download *downloadCall
}

type downloadCall struct {
	done chan struct{}
	err  error
}

// NewS3MemoryFileSource creates a lazy in-memory source. It does not contact S3;
// the first call to GetBody triggers the download.
//
// Use this constructor when the caller owns an AWS S3 API client and does not
// need to share a specially configured transfer-manager client.
func NewS3MemoryFileSource(
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
		downloader,
		bucket,
		key,
	)
}

// NewS3MemoryFileSourceWithDownloader creates a lazy source using an existing
// transfer-manager client. This form is useful for dependency injection and
// for sharing transfer-manager configuration.
func NewS3MemoryFileSourceWithDownloader(
	downloader ObjectDownloader,
	bucket string,
	key string,
) (*S3MemoryFileSource, error) {
	if downloader == nil {
		return nil, errors.New("transfer manager cannot be nil")
	}
	if bucket == "" {
		return nil, errors.New("S3 bucket cannot be empty")
	}
	if key == "" {
		return nil, errors.New("S3 object key cannot be empty")
	}

	return &S3MemoryFileSource{
		downloader: downloader,
		bucket:     bucket,
		key:        key,
	}, nil
}

// GetBody returns an independent reader over the object.
//
// The first caller downloads the object. Concurrent callers wait for the same
// attempt, but each may stop waiting by cancelling its own context. The context
// belonging to the caller that starts an attempt controls that S3 download.
// If that attempt fails, all callers waiting for it receive the same error and
// a later call may start a new attempt.
func (source *S3MemoryFileSource) GetBody(
	ctx context.Context,
) (ReadSeekerAt, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	for {
		source.mu.Lock()

		if source.cached {
			reader := bytes.NewReader(source.data)
			source.mu.Unlock()
			return reader, nil
		}

		if source.download != nil {
			call := source.download
			source.mu.Unlock()

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-call.done:
				if call.err != nil {
					return nil, call.err
				}
				// The successful caller installed the cached data before
				// closing done. Loop to create this caller's reader.
				continue
			}
		}

		call := &downloadCall{
			done: make(chan struct{}),
		}
		source.download = call
		source.mu.Unlock()

		data, err := source.downloadObject(ctx)

		source.mu.Lock()
		if err == nil {
			source.data = data
			source.cached = true
		}
		call.err = err
		source.download = nil
		close(call.done)
		source.mu.Unlock()

		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}
}

func (source *S3MemoryFileSource) downloadObject(
	ctx context.Context,
) ([]byte, error) {
	buffer := tmtypes.NewWriteAtBuffer(nil)

	_, err := source.downloader.DownloadObject(
		ctx,
		&transfermanager.DownloadObjectInput{
			Bucket:   aws.String(source.bucket),
			Key:      aws.String(source.key),
			WriterAt: buffer,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"download S3 object %q/%q into memory: %w",
			source.bucket,
			source.key,
			err,
		)
	}

	// WriteAtBuffer is no longer modified after DownloadObject returns. Keep
	// its completed byte slice without making a second object-sized copy.
	return buffer.Bytes(), nil
}

// Size returns the cached object's size. It returns zero before a successful
// GetBody call; zero is also the valid size of an empty object.
func (source *S3MemoryFileSource) Size() int64 {
	source.mu.Lock()
	defer source.mu.Unlock()

	return int64(len(source.data))
}

var (
	_ FileSource   = (*S3MemoryFileSource)(nil)
	_ ReadSeekerAt = (*bytes.Reader)(nil)
)
