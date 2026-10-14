package appside

import (
	"archive/zip"
	"io"
	"testing"
)

func zipWriter(t *testing.T, w io.Writer) *zip.Writer {
	t.Helper()
	return zip.NewWriter(w)
}
