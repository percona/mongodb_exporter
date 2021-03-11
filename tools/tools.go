//go:build tools
// +build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/reviewdog/reviewdog/cmd/reviewdog"
	_ "mvdan.cc/gofumpt"
	_ "mvdan.cc/gofumpt/gofumports"
)
