package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type LocalRepositoryCache struct {
	Parent            ActionCache
	LocalRepositories map[string]string
	CacheDirCache     map[string]string
}

func (l *LocalRepositoryCache) Fetch(ctx context.Context, cacheDir, url, ref, token string) (string, error) {
	if dest, ok := l.LocalRepositories[fmt.Sprintf("%s@%s", url, ref)]; ok {
		l.CacheDirCache[cacheDir] = dest
		return "local-repository", nil
	}
	return l.Parent.Fetch(ctx, cacheDir, url, ref, token)
}

func (l *LocalRepositoryCache) GetTarArchive(ctx context.Context, cacheDir, sha, includePrefix string) (io.ReadCloser, error) {
	if srcPath, ok := l.CacheDirCache[cacheDir]; ok {
		buf := &bytes.Buffer{}
		tw := tar.NewWriter(buf)
		defer tw.Close()
		srcPath = filepath.Clean(srcPath)
		fi, err := os.Lstat(srcPath)
		if err != nil {
			return nil, err
		}
		tc := &tarCollector{
			TarWriter: tw,
		}
		if fi.IsDir() {
			srcPrefix := filepath.Dir(srcPath)
			if !strings.HasSuffix(srcPrefix, string(filepath.Separator)) {
				srcPrefix += string(filepath.Separator)
			}
			fc := &fileCollector{
				Fs:        &defaultFs{},
				SrcPath:   srcPath,
				SrcPrefix: srcPrefix,
				Handler:   tc,
			}
			err = filepath.Walk(srcPath, fc.collectFiles(ctx, []string{}))
			if err != nil {
				return nil, err
			}
		} else {
			var f io.ReadCloser
			var linkname string
			if fi.Mode()&fs.ModeSymlink != 0 {
				linkname, err = os.Readlink(srcPath)
				if err != nil {
					return nil, err
				}
			} else {
				f, err = os.Open(srcPath)
				if err != nil {
					return nil, err
				}
				defer f.Close()
			}
			err := tc.WriteFile(fi.Name(), fi, linkname, f)
			if err != nil {
				return nil, err
			}
		}
		return io.NopCloser(buf), nil
	}
	return l.Parent.GetTarArchive(ctx, cacheDir, sha, includePrefix)
}
