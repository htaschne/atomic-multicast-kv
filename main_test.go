package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	SetProtocolLogging(false)
	os.Exit(m.Run())
}
