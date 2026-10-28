package main

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
)

var errDownloadFailed = errors.New("download failed")

type downloadStep struct {
	data    []byte
	err     error
	started chan struct{}
	release <-chan struct{}
}

type recordingDownloader struct {
	mu     sync.Mutex
	steps  []downloadStep
	calls  int
	inputs []*transfermanager.DownloadObjectInput
}

func (downloader *recordingDownloader) DownloadObject(
	ctx context.Context,
	input *transfermanager.DownloadObjectInput,
	_ ...func(*transfermanager.Options),
) (*transfermanager.DownloadObjectOutput, error) {
	downloader.mu.Lock()
	call := downloader.calls
	downloader.calls++
	downloader.inputs = append(downloader.inputs, input)

	var step downloadStep
	if call < len(downloader.steps) {
		step = downloader.steps[call]
	}
	downloader.mu.Unlock()

	if step.started != nil {
		close(step.started)
	}

	if step.release != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-step.release:
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if step.err != nil {
		return nil, step.err
	}
	if input.WriterAt == nil {
		return nil, errors.New("WriterAt is nil")
	}

	if len(step.data) > 0 {
		n, err := input.WriterAt.WriteAt(step.data, 0)
		if err != nil {
			return nil, err
		}
		if n != len(step.data) {
			return nil, io.ErrShortWrite
		}
	}

	return &transfermanager.DownloadObjectOutput{
		ContentLength: aws.Int64(int64(len(step.data))),
	}, nil
}

func (downloader *recordingDownloader) callCount() int {
	downloader.mu.Lock()
	defer downloader.mu.Unlock()

	return downloader.calls
}

func (downloader *recordingDownloader) input(index int) *transfermanager.DownloadObjectInput {
	downloader.mu.Lock()
	defer downloader.mu.Unlock()

	return downloader.inputs[index]
}

func TestNewS3MemoryFileSourceWithDownloaderValidation(t *testing.T) {
	tests := []struct {
		name       string
		downloader ObjectDownloader
		bucket     string
		key        string
	}{
		{
			name:       "nil downloader",
			downloader: nil,
			bucket:     "source-bucket",
			key:        "object.bin",
		},
		{
			name:       "empty bucket",
			downloader: &recordingDownloader{},
			bucket:     "",
			key:        "object.bin",
		},
		{
			name:       "empty key",
			downloader: &recordingDownloader{},
			bucket:     "source-bucket",
			key:        "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, err := NewS3MemoryFileSourceWithDownloader(
				test.downloader,
				test.bucket,
				test.key,
			)
			if err == nil {
				t.Fatal("NewS3MemoryFileSourceWithDownloader() error = nil")
			}
			if source != nil {
				t.Errorf(
					"NewS3MemoryFileSourceWithDownloader() source = %v, want nil",
					source,
				)
			}
		})
	}
}

func TestS3MemoryFileSourceIsLazyAndCachesData(t *testing.T) {
	want := []byte("satellite-image-data")
	downloader := &recordingDownloader{
		steps: []downloadStep{{data: want}},
	}
	source := mustMemorySource(t, downloader)

	if got := downloader.callCount(); got != 0 {
		t.Fatalf("download calls after construction = %d, want 0", got)
	}
	if got := source.Size(); got != 0 {
		t.Fatalf("Size() before download = %d, want 0", got)
	}

	first, err := source.GetBody(context.Background())
	if err != nil {
		t.Fatalf("first GetBody() error = %v", err)
	}

	firstPrefix := make([]byte, 4)
	if _, err := io.ReadFull(first, firstPrefix); err != nil {
		t.Fatalf("read first body prefix: %v", err)
	}
	if got := string(firstPrefix); got != string(want[:4]) {
		t.Errorf("first body prefix = %q, want %q", got, want[:4])
	}

	second, err := source.GetBody(context.Background())
	if err != nil {
		t.Fatalf("second GetBody() error = %v", err)
	}
	secondData, err := io.ReadAll(second)
	if err != nil {
		t.Fatalf("read second body: %v", err)
	}
	if got := string(secondData); got != string(want) {
		t.Errorf("second body = %q, want %q", got, want)
	}

	firstRemainder, err := io.ReadAll(first)
	if err != nil {
		t.Fatalf("read first body remainder: %v", err)
	}
	if got := string(firstRemainder); got != string(want[4:]) {
		t.Errorf("first body remainder = %q, want %q", got, want[4:])
	}

	if got := downloader.callCount(); got != 1 {
		t.Errorf("download calls = %d, want 1", got)
	}
	if got := source.Size(); got != int64(len(want)) {
		t.Errorf("Size() = %d, want %d", got, len(want))
	}

	input := downloader.input(0)
	if got := aws.ToString(input.Bucket); got != "source-bucket" {
		t.Errorf("download bucket = %q, want %q", got, "source-bucket")
	}
	if got := aws.ToString(input.Key); got != "path/object.bin" {
		t.Errorf("download key = %q, want %q", got, "path/object.bin")
	}
	if input.WriterAt == nil {
		t.Error("download WriterAt = nil")
	}
}

func TestS3MemoryFileSourceReadSeekerAt(t *testing.T) {
	want := []byte("0123456789")
	source := mustMemorySource(t, &recordingDownloader{
		steps: []downloadStep{{data: want}},
	})

	body, err := source.GetBody(context.Background())
	if err != nil {
		t.Fatalf("GetBody() error = %v", err)
	}

	position, err := body.Seek(5, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	if position != 5 {
		t.Errorf("Seek() position = %d, want 5", position)
	}

	sequential := make([]byte, 2)
	if _, err := io.ReadFull(body, sequential); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got := string(sequential); got != "56" {
		t.Errorf("Read() data = %q, want %q", got, "56")
	}

	positional := make([]byte, 3)
	if _, err := body.ReadAt(positional, 1); err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if got := string(positional); got != "123" {
		t.Errorf("ReadAt() data = %q, want %q", got, "123")
	}

	// ReadAt must not change the sequential cursor, which should still be 7.
	next := make([]byte, 2)
	if _, err := io.ReadFull(body, next); err != nil {
		t.Fatalf("Read() after ReadAt() error = %v", err)
	}
	if got := string(next); got != "78" {
		t.Errorf("Read() after ReadAt() = %q, want %q", got, "78")
	}
}

func TestS3MemoryFileSourceConcurrentGetBodyDownloadsOnce(t *testing.T) {
	const callers = 32

	started := make(chan struct{})
	release := make(chan struct{})
	want := []byte("shared cached data")
	downloader := &recordingDownloader{
		steps: []downloadStep{{
			data:    want,
			started: started,
			release: release,
		}},
	}
	source := mustMemorySource(t, downloader)

	start := make(chan struct{})
	errorsByCaller := make(chan error, callers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(callers)

	for range callers {
		go func() {
			defer waitGroup.Done()
			<-start

			body, err := source.GetBody(context.Background())
			if err != nil {
				errorsByCaller <- err
				return
			}

			data, err := io.ReadAll(body)
			if err != nil {
				errorsByCaller <- err
				return
			}
			if string(data) != string(want) {
				errorsByCaller <- errors.New("caller received incorrect data")
				return
			}

			errorsByCaller <- nil
		}()
	}

	close(start)
	waitForSignal(t, started, "download did not start")
	close(release)
	waitGroup.Wait()
	close(errorsByCaller)

	for err := range errorsByCaller {
		if err != nil {
			t.Errorf("concurrent GetBody() error = %v", err)
		}
	}
	if got := downloader.callCount(); got != 1 {
		t.Errorf("download calls = %d, want 1", got)
	}
}

func TestS3MemoryFileSourceWaitingCallerCanCancel(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	downloader := &recordingDownloader{
		steps: []downloadStep{{
			data:    []byte("data"),
			started: started,
			release: release,
		}},
	}
	source := mustMemorySource(t, downloader)

	firstResult := make(chan error, 1)
	go func() {
		_, err := source.GetBody(context.Background())
		firstResult <- err
	}()
	waitForSignal(t, started, "first download did not start")

	waiterContext, cancelWaiter := context.WithCancel(context.Background())
	cancelWaiter()
	_, err := source.GetBody(waiterContext)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("waiting GetBody() error = %v, want context.Canceled", err)
	}
	if got := downloader.callCount(); got != 1 {
		t.Errorf("download calls while waiter cancelled = %d, want 1", got)
	}

	close(release)
	select {
	case err := <-firstResult:
		if err != nil {
			t.Errorf("first GetBody() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first GetBody() did not finish")
	}
}

func TestS3MemoryFileSourceRetriesAfterFailure(t *testing.T) {
	downloader := &recordingDownloader{
		steps: []downloadStep{
			{err: errDownloadFailed},
			{data: []byte("retry succeeded")},
		},
	}
	source := mustMemorySource(t, downloader)

	_, err := source.GetBody(context.Background())
	if !errors.Is(err, errDownloadFailed) {
		t.Fatalf("first GetBody() error = %v, want %v", err, errDownloadFailed)
	}
	if got := source.Size(); got != 0 {
		t.Errorf("Size() after failed download = %d, want 0", got)
	}

	body, err := source.GetBody(context.Background())
	if err != nil {
		t.Fatalf("retry GetBody() error = %v", err)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read retry body: %v", err)
	}
	if got := string(data); got != "retry succeeded" {
		t.Errorf("retry body = %q, want %q", got, "retry succeeded")
	}
	if got := downloader.callCount(); got != 2 {
		t.Errorf("download calls = %d, want 2", got)
	}
}

func TestS3MemoryFileSourceRetriesAfterInitiatorCancellation(t *testing.T) {
	started := make(chan struct{})
	neverRelease := make(chan struct{})
	downloader := &recordingDownloader{
		steps: []downloadStep{
			{
				started: started,
				release: neverRelease,
			},
			{data: []byte("retry after cancellation")},
		},
	}
	source := mustMemorySource(t, downloader)

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := source.GetBody(ctx)
		result <- err
	}()
	waitForSignal(t, started, "download did not start")
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("cancelled GetBody() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled GetBody() did not finish")
	}

	body, err := source.GetBody(context.Background())
	if err != nil {
		t.Fatalf("retry GetBody() error = %v", err)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read retry body: %v", err)
	}
	if got := string(data); got != "retry after cancellation" {
		t.Errorf("retry body = %q, want %q", got, "retry after cancellation")
	}
	if got := downloader.callCount(); got != 2 {
		t.Errorf("download calls = %d, want 2", got)
	}
}

func TestS3MemoryFileSourceCachesEmptyObject(t *testing.T) {
	downloader := &recordingDownloader{
		steps: []downloadStep{{data: nil}},
	}
	source := mustMemorySource(t, downloader)

	for call := 0; call < 2; call++ {
		body, err := source.GetBody(context.Background())
		if err != nil {
			t.Fatalf("GetBody() call %d error = %v", call+1, err)
		}

		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("read body call %d: %v", call+1, err)
		}
		if len(data) != 0 {
			t.Errorf("body call %d length = %d, want 0", call+1, len(data))
		}
	}

	if got := downloader.callCount(); got != 1 {
		t.Errorf("download calls = %d, want 1", got)
	}
	if got := source.Size(); got != 0 {
		t.Errorf("Size() = %d, want 0", got)
	}
}

func TestS3MemoryFileSourceRejectsNilContext(t *testing.T) {
	downloader := &recordingDownloader{}
	source := mustMemorySource(t, downloader)

	body, err := source.GetBody(nil)
	if err == nil {
		t.Fatal("GetBody(nil) error = nil")
	}
	if body != nil {
		t.Errorf("GetBody(nil) body = %v, want nil", body)
	}
	if got := downloader.callCount(); got != 0 {
		t.Errorf("download calls = %d, want 0", got)
	}
}

func mustMemorySource(
	t *testing.T,
	downloader ObjectDownloader,
) *S3MemoryFileSource {
	t.Helper()

	source, err := NewS3MemoryFileSourceWithDownloader(
		downloader,
		"source-bucket",
		"path/object.bin",
	)
	if err != nil {
		t.Fatalf("NewS3MemoryFileSourceWithDownloader() error = %v", err)
	}
	return source
}

func waitForSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal(message)
	}
}
