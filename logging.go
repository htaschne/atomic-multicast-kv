package main

import (
	"log"
	"os"
	"sync/atomic"
)

var protocolLoggingEnabled atomic.Bool

func init() {
	protocolLoggingEnabled.Store(true)
}

var protocolLogger = log.New(os.Stderr, "", log.LstdFlags)

func SetProtocolLogging(enabled bool) {
	protocolLoggingEnabled.Store(enabled)
}

func ProtocolLoggingEnabled() bool {
	return protocolLoggingEnabled.Load()
}

func protocolLogf(format string, args ...any) {
	if protocolLoggingEnabled.Load() {
		protocolLogger.Printf(format, args...)
	}
}
