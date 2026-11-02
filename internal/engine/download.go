package engine

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/mehran9675/touchup/internal/tr"
)

// download fetches url into file, reporting the fraction done when the
// server sends a length.
func download(ctx context.Context, url, file string, progress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TouchUp")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(tr.T("download failed: {status}", "status", resp.Status))
	}
	out, err := os.Create(file)
	if err != nil {
		return err
	}
	defer out.Close()
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			done += int64(n)
			if total > 0 {
				progress(float64(done) / float64(total))
			} else {
				progress(-1)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return out.Close()
}

// archiveExt returns the archive extension of url, so extract can tell
// the format.
func archiveExt(url string) string {
	u := strings.ToLower(url)
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	for _, ext := range []string{".tar.gz", ".tgz", ".zip"} {
		if strings.HasSuffix(u, ext) {
			return ext
		}
	}
	return ".zip"
}
