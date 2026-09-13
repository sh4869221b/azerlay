# LZMA boundary fixtures

These synthetic LZMA-Alone streams contain only zero bytes: `zeros-64mib.lzma`
expands to `64 << 20` bytes and `zeros-64mib-plus-one.lzma` to `(64 << 20) + 1`.
Their headers declare unknown output size and their streams include an EOS
marker, so tests exercise the actual decompressed-output limit.

To regenerate manually, save the following as `/tmp/azerlay-lzma-fixtures.go`
and run `go run /tmp/azerlay-lzma-fixtures.go` from the repository root. This
uses the existing `github.com/ulikunitz/xz` version selected by `go.mod`.
Remove the temporary source afterward. Normal tests and CI only read the
compressed files; no decompressed files are stored.

```go
package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/ulikunitz/xz/lzma"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func main() {
	for _, fixture := range []struct {
		name string
		size int64
	}{
		{"zeros-64mib.lzma", 64 << 20},
		{"zeros-64mib-plus-one.lzma", (64 << 20) + 1},
	} {
		var compressed bytes.Buffer
		config := lzma.WriterConfig{
			DictCap: lzma.MinDictCap, SizeInHeader: false, EOSMarker: true,
		}
		writer, err := config.NewWriter(&compressed)
		if err != nil {
			panic(err)
		}
		if _, err := io.CopyN(writer, zeroReader{}, fixture.size); err != nil {
			panic(err)
		}
		if err := writer.Close(); err != nil {
			panic(err)
		}
		path := filepath.Join("internal", "profiledecode", "testdata", fixture.name)
		if err := os.WriteFile(path, compressed.Bytes(), 0o644); err != nil {
			panic(err)
		}
	}
}
```
