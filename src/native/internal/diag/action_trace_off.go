//go:build !automation

package diag

import "github.com/pablopunk/nvm/native/internal/logx"

func ActionTrace(string, logx.F) {}
