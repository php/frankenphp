// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multipart

import (
	"bufio"
	"bytes"
	"io"
	"mime"
	"net/textproto"
)

// readHeaders buffers the headers of the part, then parses them
func (p *Part) readHeaders() error {
	r := p.mr
	block := r.header[:0]
	lineStart := 0
	for {
		chunk, err := r.bufReader.ReadSlice('\n')
		block = append(block, chunk...)
		r.header = block
		if len(block) > maxHeaderSize {
			return ErrMessageTooLarge
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			// let textproto report what comes first: a malformed line or the end of the body
			return p.parseHeadersSlow(block, err)
		}

		line := trimEOL(block[lineStart:])
		if len(line) == 0 {
			break
		}
		lineStart = len(block)
	}

	if !p.parseHeaders(block) {
		return p.parseHeadersSlow(block, nil)
	}

	return nil
}

// parseHeaders parses the headers without allocating, it returns false for the ones it can't handle like textproto
func (p *Part) parseHeaders(block []byte) bool {
	var disposition []byte
	hasDisposition, hasContentType := false, false

	for lines := 0; ; lines++ {
		if lines > maxHeaders {
			return false
		}

		i := bytes.IndexByte(block, '\n')
		line := trimEOL(block[:i+1])
		block = block[i+1:]
		if len(line) == 0 {
			break
		}

		// leading spaces are continuation lines or errors
		colon := bytes.IndexByte(line, ':')
		if colon <= 0 {
			return false
		}
		key, value := line[:colon], line[colon+1:]
		for _, c := range key {
			if !validHeaderFieldByte(c) {
				return false
			}
		}
		for _, c := range value {
			if !validHeaderValueByte(c) {
				return false
			}
		}
		value = trimLWSP(value)

		switch {
		case !hasDisposition && asciiEqualFold(key, "content-disposition"):
			disposition, hasDisposition = value, true
		case !hasContentType && asciiEqualFold(key, "content-type"):
			p.contentType, hasContentType = value, true
		}
	}

	return p.parseDisposition(disposition)
}

// parseHeadersSlow parses the headers exactly like mime/multipart
func (p *Part) parseHeadersSlow(block []byte, readErr error) error {
	header, err := textproto.NewReader(bufio.NewReader(bytes.NewReader(block))).ReadMIMEHeader()
	if err == io.EOF && readErr != nil {
		err = readErr
	}
	if err != nil {
		return err
	}

	count := 0
	for _, values := range header {
		count += len(values)
	}
	if count > maxHeaders {
		return ErrMessageTooLarge
	}

	p.contentType = []byte(header.Get("Content-Type"))

	typ, params, err := mime.ParseMediaType(header.Get("Content-Disposition"))
	name, hasName := params["name"]
	if err != nil || typ != "form-data" || !hasName {
		return nil
	}

	filename, hasFilename := params["filename"]
	p.formData, p.name, p.filename, p.hasFilename = true, []byte(name), []byte(filename), hasFilename

	return nil
}

type param struct {
	key, value []byte
}

// parseDisposition parses a Content-Disposition like mime.ParseMediaType.
// It returns false for what it can't handle without allocating: RFC 2231 parameters, escapes,
// non-ASCII bytes outside of quoted strings (unicode.IsSpace could match) and many parameters.
func (p *Part) parseDisposition(v []byte) bool {
	base := v
	if i := bytes.IndexByte(v, ';'); i >= 0 {
		base = v[:i]
	}
	for _, c := range base {
		if c >= 0x80 {
			return false
		}
	}
	// invalid media types and other types are refused alike
	if !asciiEqualFold(trimLWSP(base), "form-data") {
		return true
	}

	var (
		params [maxFastParams]param
		count  int
	)

	v = v[len(base):]
	for {
		var ok bool
		if v, ok = skipSpace(v); !ok {
			return false
		}
		if len(v) == 0 {
			break
		}

		// a parameter starts with a semicolon, and the type is followed by one
		if v[0] != ';' {
			return true
		}
		rest, ok := skipSpace(v[1:])
		if !ok {
			return false
		}

		key, rest := consumeToken(rest)
		if len(key) == 0 {
			if len(rest) == 0 {
				// ignore trailing semicolons
				break
			}

			return true
		}

		if rest, ok = skipSpace(rest); !ok {
			return false
		}
		if len(rest) == 0 || rest[0] != '=' {
			return true
		}
		if rest, ok = skipSpace(rest[1:]); !ok {
			return false
		}

		var value []byte
		if len(rest) > 0 && rest[0] == '"' {
			end := bytes.IndexByte(rest[1:], '"')
			if end < 0 {
				return true
			}
			value = rest[1 : end+1]
			if bytes.IndexByte(value, '\\') >= 0 {
				return false
			}
			rest = rest[end+2:]
		} else {
			if value, rest = consumeToken(rest); len(value) == 0 {
				return true
			}
		}

		if bytes.IndexByte(key, '*') >= 0 {
			return false
		}

		duplicate := false
		for _, existing := range params[:count] {
			if bytes.EqualFold(existing.key, key) {
				if !bytes.Equal(existing.value, value) {
					return true
				}
				duplicate = true
			}
		}
		if !duplicate {
			if count == maxFastParams {
				return false
			}
			params[count] = param{key, value}
			count++
		}

		v = rest
	}

	for _, param := range params[:count] {
		switch {
		case asciiEqualFold(param.key, "name"):
			p.formData, p.name = true, param.value
		case asciiEqualFold(param.key, "filename"):
			p.hasFilename, p.filename = true, param.value
		}
	}

	return true
}

// skipSpace trims leading spaces and tabs, ok is false if a non-ASCII byte follows
func skipSpace(v []byte) (_ []byte, ok bool) {
	v = skipLWSPChar(v)

	return v, len(v) == 0 || v[0] < 0x80
}

func consumeToken(v []byte) (token, rest []byte) {
	for i, c := range v {
		if !isTokenChar(c) {
			return v[:i], v[i:]
		}
	}

	return v, nil
}

// trimEOL removes the "\n" or "\r\n" ending a line, like bufio.Reader.ReadLine
func trimEOL(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte{'\n'})

	return bytes.TrimSuffix(line, []byte{'\r'})
}

func trimLWSP(v []byte) []byte {
	v = skipLWSPChar(v)
	for len(v) > 0 && (v[len(v)-1] == ' ' || v[len(v)-1] == '\t') {
		v = v[:len(v)-1]
	}

	return v
}

// asciiEqualFold compares a token with a lowercase ASCII string
func asciiEqualFold(b []byte, lower string) bool {
	if len(b) != len(lower) {
		return false
	}
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lower[i] {
			return false
		}
	}

	return true
}

func validHeaderFieldByte(c byte) bool {
	const mask = 0 |
		(1<<(10)-1)<<'0' |
		(1<<(26)-1)<<'a' |
		(1<<(26)-1)<<'A' |
		1<<'!' |
		1<<'#' |
		1<<'$' |
		1<<'%' |
		1<<'&' |
		1<<'\'' |
		1<<'*' |
		1<<'+' |
		1<<'-' |
		1<<'.' |
		1<<'^' |
		1<<'_' |
		1<<'`' |
		1<<'|' |
		1<<'~'
	return ((uint64(1)<<c)&(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&(mask>>64)) != 0
}

func validHeaderValueByte(c byte) bool {
	const mask = 0 |
		(1<<(0x7f-0x21)-1)<<0x21 | // VCHAR: %x21-7E
		1<<0x20 | // SP: %x20
		1<<0x09 // HTAB: %x09
	return ((uint64(1)<<c)&^(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&^(mask>>64)) == 0
}

func isTokenChar(c byte) bool {
	const mask = 0 |
		(1<<(10)-1)<<'0' |
		(1<<(26)-1)<<'a' |
		(1<<(26)-1)<<'A' |
		1<<'!' |
		1<<'#' |
		1<<'$' |
		1<<'%' |
		1<<'&' |
		1<<'\'' |
		1<<'*' |
		1<<'+' |
		1<<'-' |
		1<<'.' |
		1<<'^' |
		1<<'_' |
		1<<'`' |
		1<<'{' |
		1<<'|' |
		1<<'}' |
		1<<'~'
	return ((uint64(1)<<c)&(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&(mask>>64)) != 0
}
