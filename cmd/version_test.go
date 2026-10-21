package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"jumpcode/internal/i18n"
	"jumpcode/internal/version"
)

func TestVersionDoesNotRequireProjectOrConfig(t *testing.T) {
	previousVersion, previousCommit, previousDate := version.Version, version.Commit, version.BuildDate
	previousLanguage := i18n.Language
	t.Cleanup(func() {
		version.Version, version.Commit, version.BuildDate = previousVersion, previousCommit, previousDate
		i18n.Language = previousLanguage
	})
	version.Version, version.Commit, version.BuildDate = "v0.1.0-beta.1", "abc1234-dirty", "2026-10-21T00:00:00Z"
	for _, language := range []string{"zh-CN", "en-US"} {
		for _, flag := range []string{"--version", "-v"} {
			i18n.Language = language
			root := newRootCommand()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetArgs([]string{flag})
			if err := root.Execute(); err != nil {
				t.Fatalf("%s %s 查询版本失败: %v", language, flag, err)
			}
			for _, want := range []string{"jc v0.1.0-beta.1", "abc1234-dirty", "2026-10-21T00:00:00Z", runtime.GOOS + "/" + runtime.GOARCH} {
				if !strings.Contains(output.String(), want) {
					t.Fatalf("版本信息缺少 %q: %s", want, output.String())
				}
			}
		}
	}
}
