package sdk

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
)

func TestExhaustCloseWithLogOnError_basic(t *testing.T) {
	l := hclog.NewInterceptLogger(hclog.DefaultOptions)
	l.RegisterSink(&mockSink{}) // No expectations recorded on mock.Mock so any call will fail with a panic
	exhaustCloseWithLogOnError(l, &dummyReadCloser{
		readError: io.EOF,
	})
}

func TestExhaustCloseWithLogOnError_errorsReported(t *testing.T) {
	expectedReadError := errors.New("read")
	expectedCloseError := errors.New("close")

	l := hclog.NewInterceptLogger(hclog.DefaultOptions)
	sink := &mockSink{}
	l.RegisterSink(sink)
	sink.On("Accept", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	exhaustCloseWithLogOnError(l, &dummyReadCloser{
		readError:  expectedReadError,
		closeError: expectedCloseError,
	})

	sink.AssertCalled(t, "Accept", mock.AnythingOfType("string"), hclog.Warn, "failed to exhaust reader, performance may be impacted", []interface{}{"err", expectedReadError})
	sink.AssertCalled(t, "Accept", mock.AnythingOfType("string"), hclog.Warn, "failed to close reader", []interface{}{"err", expectedCloseError})
}

func TestExhaustCloseWithLogOnError_ignoreAlreadyClosed(t *testing.T) {
	l := hclog.NewInterceptLogger(hclog.DefaultOptions)
	l.RegisterSink(&mockSink{}) // No expectations recorded on mock.Mock so any call will fail with a panic
	exhaustCloseWithLogOnError(l, &dummyReadCloser{
		readError:  io.EOF,
		closeError: os.ErrClosed,
	})
}

var _ io.ReadCloser = &dummyReadCloser{}

type dummyReadCloser struct {
	readError  error
	closeError error
}

func (d *dummyReadCloser) Read(_ []byte) (int, error) {
	return 0, d.readError
}

func (d *dummyReadCloser) Close() error {
	return d.closeError
}

var _ hclog.SinkAdapter = &mockSink{}

type mockSink struct {
	mock.Mock
}

func (m *mockSink) Accept(name string, level hclog.Level, msg string, args ...interface{}) {
	m.Called(name, level, msg, args)
}
