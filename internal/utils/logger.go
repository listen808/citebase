package utils

import (
	"io"
	"os"

	logx "github.com/listen808/citebase/internal/log"
)

// InitLogger initializes the unified logging system.
// Logs are written to both console and a rotating file (data/logs/citebase.log).
func InitLogger() {
	logx.InitFileLogger()
}

// LogWriter returns the current log writer for use by other subsystems (e.g. GORM).
func LogWriter() io.Writer {
	w := logx.Writer()
	if w == nil {
		return os.Stdout
	}
	return w
}
