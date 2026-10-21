// release 构建 Windows/Linux amd64 发布包；仅使用 Go 标准库。
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
)

type releaseInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Snapshot  bool   `json:"snapshot"`
	Dirty     bool   `json:"dirty"`
}

func main() {
	version := flag.String("version", "", "发布版本，例如 v0.1.0-beta.1")
	snapshot := flag.Bool("snapshot", false, "生成测试快照，允许未提交修改和未创建版本标签")
	flag.Parse()
	if err := release(*version, *snapshot); err != nil {
		fmt.Fprintln(os.Stderr, "发布打包失败:", err)
		os.Exit(1)
	}
}

func release(version string, snapshot bool) error {
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`).MatchString(version) {
		return errors.New("请用 --version 指定版本，例如 v0.1.0-beta.1")
	}
	root, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	commit, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	status, err := gitOutput("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if !snapshot {
		if status != "" {
			return errors.New("正式打包要求工作区干净，请先提交修改；测试打包可加 --snapshot")
		}
		tagCommit, err := gitOutput("rev-parse", "--verify", "refs/tags/"+version+"^{commit}")
		if err != nil || tagCommit != commit {
			return fmt.Errorf("正式打包要求标签 %s 指向当前提交", version)
		}
	}
	// 固定白名单，避免把本地验证草稿、用户配置或密钥放进发布包。
	paths := []string{
		"README.md", "README.en.md",
		"docs/usage.md", "docs/usage.en.md",
		"docs/architecture.md", "docs/architecture.en.md",
		"docs/development.md", "docs/development.en.md",
		"docs/releases/" + version + ".md", "docs/releases/" + version + ".en.md",
	}
	files := make(map[string][]byte)
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("读取发布文档 %s: %w", path, err)
		}
		files[path] = data
	}
	if snapshot {
		if strings.Contains(version, "-") {
			version += ".snapshot"
		} else {
			version += "-snapshot"
		}
	}
	builtAt := time.Now().UTC().Truncate(time.Second)
	info := releaseInfo{Version: version, Commit: commit, BuildDate: builtAt.Format(time.RFC3339), GoVersion: runtime.Version(), Snapshot: snapshot, Dirty: status != ""}
	if info.Dirty {
		info.Commit += "-dirty"
	}
	metadata, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	files["release.json"] = append(metadata, '\n')
	dist := filepath.Join(root, "dist")
	destination := filepath.Join(dist, version)
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("输出目录已存在或无法访问，请先检查: %s", destination)
	}
	if err := os.MkdirAll(dist, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(dist, ".release-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, platform := range []string{"windows", "linux"} {
		binary := "jc"
		extension := ".tar.gz"
		if platform == "windows" {
			binary, extension = "jc.exe", ".zip"
		}
		fmt.Printf("正在构建 %s/amd64，版本 %s\n", platform, version)
		ldflags := fmt.Sprintf("-X jumpcode/internal/version.Version=%s -X jumpcode/internal/version.Commit=%s -X jumpcode/internal/version.BuildDate=%s", info.Version, info.Commit, info.BuildDate)
		cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", filepath.Join(stage, binary), ".")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+platform, "GOARCH=amd64")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("构建 %s/amd64: %w", platform, err)
		}
		files[binary], err = os.ReadFile(filepath.Join(stage, binary))
		if err != nil {
			return err
		}
		archive := filepath.Join(stage, "jumpcode_"+version+"_"+platform+"_amd64"+extension)
		if err := writeArchive(archive, files, binary, builtAt); err != nil {
			return fmt.Errorf("生成 %s 发布包: %w", platform, err)
		}
		delete(files, binary)
		if err := os.Remove(filepath.Join(stage, binary)); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(stage, "release.json"), files["release.json"], 0644); err != nil {
		return err
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	var checksums strings.Builder
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(stage, entry.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(data), entry.Name())
	}
	if err := os.WriteFile(filepath.Join(stage, "SHA256SUMS"), []byte(checksums.String()), 0644); err != nil {
		return err
	}
	if err := os.Rename(stage, destination); err != nil {
		return fmt.Errorf("保存发布目录: %w", err)
	}
	fmt.Printf("发布产物已生成: %s\n", destination)
	return nil
}

func gitOutput(args ...string) (string, error) {
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func writeArchive(path string, files map[string][]byte, binary string, builtAt time.Time) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	if strings.HasSuffix(path, ".zip") {
		writer := zip.NewWriter(f)
		defer func() { err = errors.Join(err, writer.Close(), f.Close()) }()
		for _, name := range names {
			header := &zip.FileHeader{Name: name, Method: zip.Deflate}
			header.SetModTime(builtAt)
			header.SetMode(0644)
			if name == binary {
				header.SetMode(0755)
			}
			entry, err := writer.CreateHeader(header)
			if err != nil {
				return err
			}
			if _, err := entry.Write(files[name]); err != nil {
				return err
			}
		}
		return nil
	}
	compressed := gzip.NewWriter(f)
	writer := tar.NewWriter(compressed)
	defer func() { err = errors.Join(err, writer.Close(), compressed.Close(), f.Close()) }()
	for _, name := range names {
		mode := int64(0644)
		if name == binary {
			mode = 0755
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(files[name])), ModTime: builtAt}); err != nil {
			return err
		}
		if _, err := writer.Write(files[name]); err != nil {
			return err
		}
	}
	return nil
}
