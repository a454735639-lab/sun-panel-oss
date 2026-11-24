package file

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ZipFile struct{}

// Create 把多个「压缩包内路径 -> 本地路径」条目打包成 zip。
// 本地路径是目录时递归收录其下所有常规文件。
func (z *ZipFile) Create(items map[string]string, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for arc, src := range items {
		if err := zipWalk(w, arc, src); err != nil {
			return err
		}
	}
	return nil
}

func zipWalk(w *zip.Writer, arc, src string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if !info.IsDir() {
		return zipAddFile(w, arc, src)
	}
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil || !fi.Mode().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		return zipAddFile(w, filepath.Join(arc, rel), p)
	})
}

func zipAddFile(w *zip.Writer, arc, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	hdr, err := zip.FileInfoHeader(mustStat(src))
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(arc)
	hdr.Method = zip.Deflate

	out, err := w.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return err
}

func mustStat(p string) os.FileInfo {
	fi, err := os.Stat(p)
	if err != nil {
		panic(err) // 调用方已 Stat 过，理论不可达
	}
	return fi
}

// Extract 解压 zip 到 destDir，保留包内相对路径；拒绝包含 .. 的条目。
func (z *ZipFile) Extract(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return err
	}

	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("illegal zip entry: %s", f.Name)
		}
		target := filepath.Join(destDir, name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, os.ModePerm); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
			return err
		}

		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
