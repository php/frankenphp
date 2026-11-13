package multipart

import (
	"bytes"
	"io"
	"mime"
	stdmultipart "mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type partResult struct {
	formData    bool
	name        string
	filename    string
	hasFilename bool
	contentType string
	body        string
	bodyErr     bool
}

type parseResult struct {
	parts []partResult
	err   bool
}

// parseStd parses like the FrankenPHP form parser did with mime/multipart
func parseStd(body []byte, boundary string, skip uint64) parseResult {
	var res parseResult
	r := stdmultipart.NewReader(bytes.NewReader(body), boundary)
	for i := 0; ; i++ {
		p, err := r.NextRawPart()
		if err == io.EOF {
			return res
		}
		if err != nil {
			res.err = true

			return res
		}

		var pr partResult
		typ, params, err := mime.ParseMediaType(p.Header.Get("Content-Disposition"))
		if name, ok := params["name"]; err == nil && typ == "form-data" && ok {
			pr.formData, pr.name = true, name
			pr.filename, pr.hasFilename = params["filename"]
		}
		pr.contentType = p.Header.Get("Content-Type")
		if skip&(1<<(i%64)) == 0 {
			data, err := io.ReadAll(p)
			pr.body, pr.bodyErr = string(data), err != nil
		}
		res.parts = append(res.parts, pr)
	}
}

// chunkReader returns at most size bytes per read, like a network body
type chunkReader struct {
	r    io.Reader
	size int
}

func (c chunkReader) Read(p []byte) (int, error) {
	return c.r.Read(p[:min(len(p), c.size)])
}

func parseFork(r *Reader, body []byte, boundary string, skip uint64, chunk uint8) parseResult {
	var res parseResult
	var src io.Reader = bytes.NewReader(body)
	if chunk > 0 {
		src = chunkReader{src, int(chunk)}
	}
	r.Reset(src, boundary)
	for i := 0; ; i++ {
		p, err := r.NextPart()
		if err == io.EOF {
			return res
		}
		if err != nil {
			res.err = true

			return res
		}

		var pr partResult
		name, filename, hasFilename, ok := p.FormData()
		if ok {
			pr.formData, pr.name, pr.filename, pr.hasFilename = true, string(name), string(filename), hasFilename
		}
		pr.contentType = string(p.ContentType())
		if skip&(1<<(i%64)) == 0 {
			data, err := io.ReadAll(p)
			pr.body, pr.bodyErr = string(data), err != nil
		}
		res.parts = append(res.parts, pr)
	}
}

var seeds = []string{
	"--b\r\nContent-Disposition: form-data; name=\"a\"\r\n\r\nvalue\r\n--b\r\nContent-Disposition: form-data; name=\"f\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\ncontent\r\n--b--\r\n",
	"preamble\r\n--b\r\ncontent-disposition: FORM-DATA ; NAME = a ; name=a;\r\n\r\n\r\n--b--",
	"--b\nContent-Disposition: form-data; name=a\n\nlf only\n--b--\n",
	"--b\r\nContent-Disposition: form-data; name=\"caf\xc3\xa9\"; filename=\"\xc3\xa9t\xc3\xa9.txt\"\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name*=UTF-8''%C3%A9\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=\"a\\\"b\"; filename=\"C:\\dir\\a.txt\"\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data;\r\n name=folded\r\n\r\nx\r\n--b--\r\n",
	"--b\r\n Content-Disposition: form-data; name=a\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition : form-data; name=a\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a; name=b\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a; ;\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=\"unterminated\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: attachment; name=a\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data\xc2\xa0; name=a\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data;\xc2\x85name=a\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\r\nContent-Disposition: form-data; name=b\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\r\nX-Bad\x01: 1\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\r\nX: \x7f\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\r\nno colon\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\r\nbroken\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\r\n\r\nbody without end",
	"--b\r\nContent-Disposition: form-data; name=a\r\n\r\n--b\r\nContent-Disposition: form-data; name=b\r\n\r\n--b--",
	"--b \t\r\nContent-Disposition: form-data; name=a\r\n\r\nx\r\n--bb\r\n\r\n--b--  \r\nepilogue",
	"--b\r\nContent-Disposition: form-data; name=a\r\n\r\nx\r\n--b\r\njunk\r\n",
	"--b\r\n\r\nno headers\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=\"\"\r\n\r\nempty name\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a; filename=\"\"\r\nContent-Type: application/octet-stream; charset=x\r\n\r\n\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a; p1=1; p2=2; p3=3; p4=4; p5=5; p6=6; p7=7; p8=8\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a; filename=\"a;b c\"\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: form-data; name=a\"b\r\n\r\nx\r\n--b--\r\n",
	"--b\r\nContent-Disposition: \tform-data\t;\tname\t=\t\"a\"\t\r\n\r\nx\r\n--b--\r\n",
}

func FuzzReader(f *testing.F) {
	for _, s := range seeds {
		f.Add([]byte(s), "b", uint64(0), uint8(0))
		f.Add([]byte(s), "b", uint64(0b1010), uint8(1))
		f.Add([]byte(s), "b", uint64(0), uint8(7))
	}
	f.Add([]byte("--"+strings.Repeat("b", 5000)+"\r\nContent-Disposition: form-data; name=a\r\n\r\nx\r\n--"+strings.Repeat("b", 5000)+"--\r\n"), strings.Repeat("b", 5000), uint64(0), uint8(0))

	r := new(Reader)
	f.Fuzz(func(t *testing.T, body []byte, boundary string, skip uint64, chunk uint8) {
		// the stdlib accepts larger headers
		if len(body) > maxHeaderSize {
			t.Skip()
		}

		want := parseStd(body, boundary, skip)
		got := parseFork(r, body, boundary, skip, chunk)
		if !assert.Equal(t, want, got) {
			t.Logf("body: %q", body)
		}
	})
}

func TestReaderLargeBodies(t *testing.T) {
	big := strings.Repeat("x\r\n-", 10000)
	body := "--b\r\nContent-Disposition: form-data; name=\"a\"; filename=\"f\"\r\n\r\n" + big + "\r\n--b\r\nContent-Disposition: form-data; name=\"" + strings.Repeat("n", 5000) + "\"\r\n\r\n" + big + "\r\n--b--\r\n"

	r := new(Reader)
	for skip := range uint64(4) {
		for _, chunk := range []uint8{0, 1, 100} {
			assert.Equal(t, parseStd([]byte(body), "b", skip), parseFork(r, []byte(body), "b", skip, chunk))
		}
	}
}

func TestReaderHeaderTooLarge(t *testing.T) {
	body := "--b\r\nContent-Disposition: form-data; name=a\r\nX: " + strings.Repeat("x", maxHeaderSize) + "\r\n\r\nx\r\n--b--\r\n"

	var r Reader
	r.Reset(strings.NewReader(body), "b")
	_, err := r.NextPart()
	require.ErrorIs(t, err, ErrMessageTooLarge)
}

func TestReaderDoesNotAllocate(t *testing.T) {
	var body strings.Builder
	for range 100 {
		body.WriteString("--boundary\r\nContent-Disposition: form-data; name=\"field\"\r\n\r\nvalue\r\n")
	}
	body.WriteString("--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\ncontent\r\n--boundary--\r\n")
	src := strings.NewReader(body.String())

	var r Reader
	buf := make([]byte, 64)
	parse := func() {
		src.Reset(body.String())
		r.Reset(src, "boundary")
		for {
			p, err := r.NextPart()
			if err == io.EOF {
				return
			}
			require.NoError(t, err)
			_, _, _, ok := p.FormData()
			require.True(t, ok)
			for {
				if _, err := p.Read(buf); err != nil {
					break
				}
			}
		}
	}

	parse()
	assert.Zero(t, testing.AllocsPerRun(10, parse))
}
