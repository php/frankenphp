package frankenphp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParseMultipart(b *testing.B) {
	var buf bytes.Buffer
	for i := range 200 {
		fmt.Fprintf(&buf, "--b\r\nContent-Disposition: form-data; name=\"field%d\"\r\n\r\n%s\r\n", i, strings.Repeat("v", 32))
	}
	buf.WriteString("--b--\r\n")
	body := buf.Bytes()

	limits := formLimits{postMaxSize: 64 << 20, maxInputVars: 10000, maxFileUploads: 100, maxMultipartBodyParts: -1, fileUploads: true, tmpDir: b.TempDir()}
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		fb := formBuffersPool.Get().(*formBuffers)
		if _, rej := parseMultipart(fb, bytes.NewReader(body), "multipart/form-data; boundary=b", int64(len(body)), limits); rej != nil {
			b.Fatal(rej.detail)
		}
		removeUploads(fb.uploaded)
		fb.release()
	}
}
