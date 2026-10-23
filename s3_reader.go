// Package s3reader provides random and sequential access to an Amazon S3
// object without downloading the complete object first.
package s3reader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client is the subset of the AWS S3 client used by Reader.
// *s3.Client implements this interface.
type S3Client interface {
	HeadObject(
		context.Context,
		*s3.HeadObjectInput,
		...func(*s3.Options),
	) (*s3.HeadObjectOutput, error)

	GetObject(
		context.Context,
		*s3.GetObjectInput,
		...func(*s3.Options),
	) (*s3.GetObjectOutput, error)
}

// Reader provides lazy random and sequential access to one S3 object.
//
// Read and Seek share a logical cursor and are protected by a mutex. ReadAt
// does not use that cursor and is safe for concurrent calls. Each Read or
// ReadAt operation issues one ranged S3 GetObject request, so callers making
// many small reads should normally add a block cache above Reader.
//
// The context passed to New is used for the lifetime of Reader. Cancelling it
// cancels current and subsequent S3 requests made by Reader.
type Reader struct {
	ctx    context.Context
	client S3Client
	bucket string
	key    string
	size   int64
	etag   *string

	mu     sync.Mutex
	offset int64
}

// New creates a Reader for the current version of the named S3 object.
// It performs one HeadObject request to obtain the object size and ETag.
// Subsequent ranged reads use If-Match so they fail rather than combine data
// from different object versions if the key is replaced while being read.
func New(
	ctx context.Context,
	client S3Client,
	bucket string,
	key string,
) (*Reader, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}
	if client == nil {
		return nil, errors.New("S3 client cannot be nil")
	}
	if bucket == "" {
		return nil, errors.New("S3 bucket cannot be empty")
	}
	if key == "" {
		return nil, errors.New("S3 object key cannot be empty")
	}

	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("head S3 object %q/%q: %w", bucket, key, err)
	}
	if head.ContentLength == nil {
		return nil, errors.New("S3 HeadObject response has no content length")
	}
	if *head.ContentLength < 0 {
		return nil, fmt.Errorf(
			"S3 HeadObject response has negative content length: %d",
			*head.ContentLength,
		)
	}

	var etag *string
	if aws.ToString(head.ETag) != "" {
		value := aws.ToString(head.ETag)
		etag = &value
	}

	return &Reader{
		ctx:    ctx,
		client: client,
		bucket: bucket,
		key:    key,
		size:   *head.ContentLength,
		etag:   etag,
	}, nil
}

// Size returns the object's size in bytes as observed by New.
func (r *Reader) Size() int64 {
	return r.size
}

// Read reads from the current logical cursor and advances it by the number of
// bytes returned. Calls to Read and Seek are serialized with each other.
func (r *Reader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, err := r.ReadAt(p, r.offset)
	r.offset += int64(n)
	return n, err
}

// ReadAt reads len(p) bytes beginning at offset without changing the logical
// cursor. It is safe to call ReadAt concurrently, including concurrently with
// Read or Seek.
func (r *Reader) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, errors.New("negative read offset")
	}
	if len(p) == 0 {
		return 0, nil
	}
	if offset >= r.size {
		return 0, io.EOF
	}

	length := min(int64(len(p)), r.size-offset)
	end := offset + length - 1
	rangeValue := fmt.Sprintf("bytes=%d-%d", offset, end)

	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.key),
		Range:  aws.String(rangeValue),
	}
	if r.etag != nil {
		input.IfMatch = r.etag
	}

	output, err := r.client.GetObject(r.ctx, input)
	if err != nil {
		return 0, fmt.Errorf(
			"get S3 object %q/%q range %s: %w",
			r.bucket,
			r.key,
			rangeValue,
			err,
		)
	}
	if output.Body == nil {
		return 0, errors.New("S3 GetObject response has no body")
	}
	defer output.Body.Close()

	n, err := io.ReadFull(output.Body, p[:length])
	if err != nil {
		return n, fmt.Errorf(
			"read S3 object %q/%q range %s: %w",
			r.bucket,
			r.key,
			rangeValue,
			err,
		)
	}

	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Seek sets the logical cursor relative to the start, current cursor, or end
// of the object. Seeking beyond the end is allowed; a subsequent Read returns
// io.EOF. Seeking before the start is rejected.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var base int64
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = r.offset
	case io.SeekEnd:
		base = r.size
	default:
		return 0, fmt.Errorf("invalid seek origin: %d", whence)
	}

	absolute, ok := addInt64(base, offset)
	if !ok {
		return 0, errors.New("seek offset overflow")
	}
	if absolute < 0 {
		return 0, errors.New("negative seek position")
	}

	r.offset = absolute
	return absolute, nil
}

func addInt64(left, right int64) (int64, bool) {
	result := left + right
	if right > 0 && result < left {
		return 0, false
	}
	if right < 0 && result > left {
		return 0, false
	}
	return result, true
}

var (
	_ io.Reader     = (*Reader)(nil)
	_ io.ReaderAt   = (*Reader)(nil)
	_ io.Seeker     = (*Reader)(nil)
	_ io.ReadSeeker = (*Reader)(nil)
)
