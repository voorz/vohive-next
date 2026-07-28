package sipgw

import (
	"github.com/voorz/vohive/pkg/logger"
)

func shouldLogSIPRaw() bool {
	return logger.ShouldLogSIPRaw()
}

func redactSIPRaw(raw string) string {
	return logger.RedactSIPRaw(raw)
}
