package logs

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Archive writes the given LogFiles into a tar.gz at destPath.
// Compressed (.gz) files are included as-is without re-compressing.
func Archive(files []LogFile, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, lf := range files {
		if err := addToArchive(tw, lf); err != nil {
			return err
		}
	}
	return nil
}

func addToArchive(tw *tar.Writer, lf LogFile) error {
	if lf.SSH != nil {
		return addSSHToArchive(tw, lf)
	}

	info, err := os.Stat(lf.Path)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:    filepath.Base(lf.Path),
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(lf.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}

func addSSHToArchive(tw *tar.Writer, lf LogFile) error {
	content, err := Read(lf)
	if err != nil {
		return err
	}

	data := []byte(content)
	hdr := &tar.Header{
		Name:    lf.Name,
		Size:    int64(len(data)),
		Mode:    int64(lf.Mode),
		ModTime: lf.ModTime,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	_, err = io.Copy(tw, strings.NewReader(content))
	return err
}
