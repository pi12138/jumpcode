// Package version 提供构建时注入的版本信息，普通源码构建默认为开发版本。
package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s, %s/%s)", Version, Commit, BuildDate, runtime.GOOS, runtime.GOARCH)
}
