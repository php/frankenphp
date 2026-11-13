// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package multipart is a fork of the mime/multipart Reader of Go 1.27.4 that doesn't allocate per part.
//
// Boundaries are scanned like the standard library does. Headers are only parsed for the
// Content-Disposition and Content-Type of multipart/form-data, without allocating in the common case:
// unusual headers are handed to net/textproto and mime, so they are parsed exactly like with
// mime/multipart, except that headers larger than 64 KiB are refused.
package multipart

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// This constant needs to be at least 76 for this package to work correctly.
// This is because \r\n--separator_of_len_70- would fill the buffer and it
// wouldn't be safe to consume a single byte from it.
const peekBufferSize = 4096

// maxHeaderSize bounds the headers of a part: they are buffered before being parsed
const maxHeaderSize = 64 << 10

// maxHeaders is the default limit of mime/multipart
const maxHeaders = 10000

// maxFastParams bounds the Content-Disposition parameters parsed without allocating
const maxFastParams = 8

var ErrMessageTooLarge = errors.New("multipart: message too large")

// Reader is an iterator over the parts of a multipart body. Its memory is reused by [Reader.Reset].
type Reader struct {
	bufReader *bufio.Reader
	src       stickyErrorReader

	part      Part
	inPart    bool
	partsRead int

	boundary         []byte // "\r\n--boundary--"
	nl               []byte // "\r\n" or "\n" (set after seeing first boundary line)
	nlDashBoundary   []byte // nl + "--boundary"
	dashBoundaryDash []byte // "--boundary--"
	dashBoundary     []byte // "--boundary"

	header []byte
}

// Reset makes r read the parts of src, separated by boundary.
func (r *Reader) Reset(src io.Reader, boundary string) {
	r.src = stickyErrorReader{r: src}
	if r.bufReader == nil {
		r.bufReader = bufio.NewReaderSize(&r.src, peekBufferSize)
	} else {
		r.bufReader.Reset(&r.src)
	}

	b := append(append(append(r.boundary[:0], "\r\n--"...), boundary...), "--"...)
	r.boundary = b
	r.nl = b[:2]
	r.nlDashBoundary = b[:len(b)-2]
	r.dashBoundaryDash = b[2:]
	r.dashBoundary = b[2 : len(b)-2]

	r.part = Part{}
	r.inPart = false
	r.partsRead = 0
	if cap(r.header) > maxHeaderSize {
		r.header = nil
	}
}

// stickyErrorReader is an io.Reader which never calls Read on its
// underlying Reader once an error has been seen. (the io.Reader
// interface's contract promises nothing about the return values of
// Read calls after an error, yet this package does do multiple Reads
// after error)
type stickyErrorReader struct {
	r   io.Reader
	err error
}

func (r *stickyErrorReader) Read(p []byte) (n int, _ error) {
	if r.err != nil {
		return 0, r.err
	}
	n, r.err = r.r.Read(p)
	return n, r.err
}

// A Part is a single part of a multipart body. It is only valid until the next call to [Reader.NextPart].
type Part struct {
	mr *Reader

	n       int   // known data bytes waiting in mr.bufReader
	total   int64 // total data bytes read already
	err     error // error to return when n == 0
	readErr error // read error observed from mr.bufReader

	formData    bool
	hasFilename bool
	name        []byte
	filename    []byte
	contentType []byte
}

// FormData returns the name and filename parameters of a "form-data" Content-Disposition.
// ok is false when the header is missing, invalid, of another type or without a name.
func (p *Part) FormData() (name, filename []byte, hasFilename, ok bool) {
	return p.name, p.filename, p.hasFilename, p.formData
}

// ContentType returns the value of the first Content-Type header.
func (p *Part) ContentType() []byte {
	return p.contentType
}

// fill reads into the buffer until it identifies some data to return,
// or finds a reason to stop (boundary or read error).
func (p *Part) fill() {
	br := p.mr.bufReader
	for p.n == 0 && p.err == nil {
		peek, _ := br.Peek(br.Buffered())
		p.n, p.err = scanUntilBoundary(peek, p.mr.dashBoundary, p.mr.nlDashBoundary, p.total, p.readErr)
		if p.n == 0 && p.err == nil {
			// Force buffered I/O to read more into buffer.
			_, p.readErr = br.Peek(len(peek) + 1)
			if p.readErr == io.EOF {
				p.readErr = io.ErrUnexpectedEOF
			}
		}
	}
}

// Read reads the body of a part, after its headers and before the next part (if any) begins.
func (p *Part) Read(d []byte) (n int, err error) {
	p.fill()
	if p.n == 0 {
		return 0, p.err
	}
	n, _ = p.mr.bufReader.Read(d[:min(len(d), p.n)])
	p.total += int64(n)
	p.n -= n
	if p.n == 0 {
		return n, p.err
	}
	return n, nil
}

func (p *Part) discard() {
	for {
		p.fill()
		if p.n == 0 {
			return
		}
		n, _ := p.mr.bufReader.Discard(p.n)
		p.total += int64(n)
		p.n -= n
	}
}

// scanUntilBoundary scans buf to identify how much of it can be safely
// returned as part of the Part body.
// dashBoundary is "--boundary".
// nlDashBoundary is "\r\n--boundary" or "\n--boundary", depending on what mode we are in.
// The comments below (and the name) assume "\n--boundary", but either is accepted.
// total is the number of bytes read out so far. If total == 0, then a leading "--boundary" is recognized.
// readErr is the read error, if any, that followed reading the bytes in buf.
// scanUntilBoundary returns the number of data bytes from buf that can be
// returned as part of the Part body and also the error to return (if any)
// once those data bytes are done.
func scanUntilBoundary(buf, dashBoundary, nlDashBoundary []byte, total int64, readErr error) (int, error) {
	if total == 0 {
		// At beginning of body, allow dashBoundary.
		if bytes.HasPrefix(buf, dashBoundary) {
			switch matchAfterPrefix(buf, dashBoundary, readErr) {
			case -1:
				return len(dashBoundary), nil
			case 0:
				return 0, nil
			case +1:
				return 0, io.EOF
			}
		}
		if bytes.HasPrefix(dashBoundary, buf) {
			return 0, readErr
		}
	}

	// Search for "\n--boundary".
	if i := bytes.Index(buf, nlDashBoundary); i >= 0 {
		switch matchAfterPrefix(buf[i:], nlDashBoundary, readErr) {
		case -1:
			return i + len(nlDashBoundary), nil
		case 0:
			return i, nil
		case +1:
			return i, io.EOF
		}
	}
	if bytes.HasPrefix(nlDashBoundary, buf) {
		return 0, readErr
	}

	// Otherwise, anything up to the final \n is not part of the boundary
	// and so must be part of the body.
	// Also if the section from the final \n onward is not a prefix of the boundary,
	// it too must be part of the body.
	i := bytes.LastIndexByte(buf, nlDashBoundary[0])
	if i >= 0 && bytes.HasPrefix(nlDashBoundary, buf[i:]) {
		return i, nil
	}
	return len(buf), readErr
}

// matchAfterPrefix checks whether buf should be considered to match the boundary.
// The prefix is "--boundary" or "\r\n--boundary" or "\n--boundary",
// and the caller has verified already that bytes.HasPrefix(buf, prefix) is true.
//
// matchAfterPrefix returns +1 if the buffer does match the boundary,
// meaning the prefix is followed by a double dash, space, tab, cr, nl,
// or end of input.
// It returns -1 if the buffer definitely does NOT match the boundary,
// meaning the prefix is followed by some other character.
// For example, "--foobar" does not match "--foo".
// It returns 0 more input needs to be read to make the decision,
// meaning that len(buf) == len(prefix) and readErr == nil.
func matchAfterPrefix(buf, prefix []byte, readErr error) int {
	if len(buf) == len(prefix) {
		if readErr != nil {
			return +1
		}
		return 0
	}
	c := buf[len(prefix)]

	if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
		return +1
	}

	// Try to detect boundaryDash
	if c == '-' {
		if len(buf) == len(prefix)+1 {
			if readErr != nil {
				// Prefix + "-" does not match
				return -1
			}
			return 0
		}
		if buf[len(prefix)+1] == '-' {
			return +1
		}
	}

	return -1
}

// NextPart returns the next part in the multipart or an error.
// When there are no more parts, the error [io.EOF] is returned.
// Like [mime/multipart.Reader.NextRawPart], the Content-Transfer-Encoding is ignored.
func (r *Reader) NextPart() (*Part, error) {
	if r.inPart {
		r.part.discard()
		r.inPart = false
	}
	if len(r.dashBoundary) == 2 {
		return nil, errors.New("multipart: boundary is empty")
	}
	expectNewPart := false
	for {
		line, err := r.bufReader.ReadSlice('\n')

		if err == io.EOF && r.isFinalBoundary(line) {
			// If the buffer ends in "--boundary--" without the
			// trailing "\r\n", ReadSlice will return an error
			// (since it's missing the '\n'), but this is a valid
			// multipart EOF so we need to return io.EOF instead of
			// a fmt-wrapped one.
			return nil, io.EOF
		}
		if err != nil {
			return nil, fmt.Errorf("multipart: NextPart: %w", err)
		}

		if r.isBoundaryDelimiterLine(line) {
			r.partsRead++
			r.part = Part{mr: r}
			if err := r.part.readHeaders(); err != nil {
				return nil, err
			}
			r.inPart = true
			return &r.part, nil
		}

		if r.isFinalBoundary(line) {
			// Expected EOF
			return nil, io.EOF
		}

		if expectNewPart {
			return nil, fmt.Errorf("multipart: expecting a new Part; got line %q", string(line))
		}

		if r.partsRead == 0 {
			// skip line
			continue
		}

		// Consume the "\n" or "\r\n" separator between the
		// body of the previous part and the boundary line we
		// now expect will follow. (either a new part or the
		// end boundary)
		if bytes.Equal(line, r.nl) {
			expectNewPart = true
			continue
		}

		return nil, fmt.Errorf("multipart: unexpected line in Next(): %q", line)
	}
}

// isFinalBoundary reports whether line is the final boundary line
// indicating that all parts are over.
// It matches `^--boundary--[ \t]*(\r\n)?$`
func (r *Reader) isFinalBoundary(line []byte) bool {
	if !bytes.HasPrefix(line, r.dashBoundaryDash) {
		return false
	}
	rest := line[len(r.dashBoundaryDash):]
	rest = skipLWSPChar(rest)
	return len(rest) == 0 || bytes.Equal(rest, r.nl)
}

func (r *Reader) isBoundaryDelimiterLine(line []byte) (ret bool) {
	// https://tools.ietf.org/html/rfc2046#section-5.1
	//   The boundary delimiter line is then defined as a line
	//   consisting entirely of two hyphen characters ("-",
	//   decimal value 45) followed by the boundary parameter
	//   value from the Content-Type header field, optional linear
	//   whitespace, and a terminating CRLF.
	if !bytes.HasPrefix(line, r.dashBoundary) {
		return false
	}
	rest := line[len(r.dashBoundary):]
	rest = skipLWSPChar(rest)

	// On the first part, see our lines are ending in \n instead of \r\n
	// and switch into that mode if so. This is a violation of the spec,
	// but occurs in practice.
	if r.partsRead == 0 && len(rest) == 1 && rest[0] == '\n' {
		r.nl = r.nl[1:]
		r.nlDashBoundary = r.nlDashBoundary[1:]
	}
	return bytes.Equal(rest, r.nl)
}

// skipLWSPChar returns b with leading spaces and tabs removed.
// RFC 822 defines:
//
//	LWSP-char = SPACE / HTAB
func skipLWSPChar(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}
