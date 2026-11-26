package fsutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WriteOptions controls atomic file replacement behavior.
type WriteOptions struct {
	Perm          os.FileMode
	DirPerm       os.FileMode
	EnsureDir     bool
	PreserveMode  bool
	FollowSymlink bool
	TempPattern   string
}

// WriteFileAtomic atomically writes data to path via a same-directory temp file, fsync, and rename.
func WriteFileAtomic(path string, data []byte, opts WriteOptions) error {
	return WriteStreamAtomic(path, opts, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// WriteStreamAtomic atomically writes streamed content to path via a same-directory temp file.
func WriteStreamAtomic(path string, opts WriteOptions, write func(io.Writer) error) error {
	target := path
	if opts.FollowSymlink {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			target = resolved
		}
	}

	perm := opts.Perm
	if perm == 0 {
		perm = 0600
	}
	if opts.PreserveMode {
		if info, err := os.Stat(target); err == nil {
			perm = info.Mode().Perm()
		}
	}

	dir := filepath.Dir(target)
	if opts.EnsureDir {
		dirPerm := opts.DirPerm
		if dirPerm == 0 {
			dirPerm = 0700
		}
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return err
		}
	}

	pattern := opts.TempPattern
	if pattern == "" {
		pattern = ".mihosh-tmp-*"
	}
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := write(tmp); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("replace target file: %w", err)
	}
	return nil
}

// MarshalYAMLIndent2 serializes v with 2-space indentation matching Mihomo YAML conventions.
func MarshalYAMLIndent2(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
