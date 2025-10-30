package shared

import (
	"fmt"
	"io"
	"os"
)

// Reporter emits formatted executor events to an underlying sink.
type Reporter interface {
	Printf(format string, args ...any)
}

type writerReporter struct {
	writer io.Writer
}

// NewWriterReporter constructs a Reporter that writes to the provided io.Writer.
func NewWriterReporter(writer io.Writer) Reporter {
	if writer == nil || writer == io.Discard {
		writer = os.Stdout
	}
	return writerReporter{writer: writer}
}

func (reporter writerReporter) Printf(format string, args ...any) {
	if reporter.writer == nil {
		return
	}
	fmt.Fprintf(reporter.writer, format, args...)
}
