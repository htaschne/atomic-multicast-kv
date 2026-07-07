#!/bin/sh
set -eu

go test -bench=. -benchmem ./...
