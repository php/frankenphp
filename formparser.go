package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/multipart"
)

// UPLOAD_ERR_* codes, rfc1867.h only exports them since PHP 8.3
// TODO: switch to C.PHP_UPLOAD_ERROR_* when dropping PHP 8.2 support
const (
	uploadErrIniSize   = 1
	uploadErrFormSize  = 2
	uploadErrNoFile    = 4
	uploadErrNoTmpDir  = 6
	uploadErrCantWrite = 7
)

type formEntry struct {
	kind   C.frankenphp_form_entry_kind
	name   []byte
	value  []byte
	number int64
}

type formLimits struct {
	postMaxSize           int64
	maxInputVars          int64
	maxFileUploads        int64
	maxMultipartBodyParts int64
	uploadMaxFilesize     int64
	fileUploads           bool
	tmpDir                string
}

// formRejection refuses a whole body: detail goes to logs and request_parse_body() exceptions,
// not to the client
type formRejection struct {
	rejected ErrRejected
	detail   string
}

func tooLarge(format string, args ...any) *formRejection {
	return &formRejection{ErrRequestBodyTooLarge, fmt.Sprintf(format, args...)}
}

func malformed(format string, args ...any) *formRejection {
	return &formRejection{ErrMalformedRequestBody, fmt.Sprintf(format, args...)}
}

func warning(format string, args ...any) formEntry {
	return formEntry{kind: C.FRANKENPHP_FORM_WARNING, name: fmt.Appendf(nil, format, args...)}
}

// formBuffers holds the memory of a parsed body until it's copied to C, then goes back to a pool
type formBuffers struct {
	// registered first, so PHP deletes them even if registering the rest bails out
	uploaded []formEntry
	entries  []formEntry
	// names and values share one buffer to spare the GC
	data       []byte
	normalized []byte
	protected  map[string]struct{}
	limited    io.LimitedReader
	multipart  multipart.Reader
}

var formBuffersPool = sync.Pool{New: func() any {
	return &formBuffers{data: make([]byte, 0, 4096), protected: make(map[string]struct{})}
}}

func (b *formBuffers) release() {
	// don't keep the memory of large bodies around
	if cap(b.data) > 1<<20 || cap(b.entries) > 4096 {
		return
	}

	clear(b.uploaded)
	clear(b.entries)
	clear(b.protected)
	b.uploaded, b.entries, b.data = b.uploaded[:0], b.entries[:0], b.data[:0]
	formBuffersPool.Put(b)
}

// bytes stores the concatenation of parts in the shared buffer
func (b *formBuffers) bytes(parts ...string) []byte {
	start := len(b.data)
	for _, p := range parts {
		b.data = append(b.data, p...)
	}

	return b.data[start:len(b.data):len(b.data)]
}

// parseMultipart parses an RFC 7578 multipart/form-data body into b and stores uploaded files in limits.tmpDir.
// On rejection, the caller must remove the uploaded files.
func parseMultipart(b *formBuffers, body io.Reader, contentType string, contentLength int64, limits formLimits) (int64, *formRejection) {
	if limits.postMaxSize > 0 && contentLength > limits.postMaxSize {
		return 0, tooLarge("POST Content-Length of %d bytes exceeds the limit of %d bytes", contentLength, limits.postMaxSize)
	}

	_, params, err := mime.ParseMediaType(contentType)
	boundary := params["boundary"]
	if err != nil || boundary == "" {
		return 0, malformed("Missing boundary in multipart/form-data POST data")
	}

	// like rfc1867.c, stop once more than post_max_size bytes were read
	readLimit := int64(math.MaxInt64)
	if limits.postMaxSize > 0 && limits.postMaxSize < math.MaxInt64 {
		readLimit = limits.postMaxSize + 1
	}
	b.limited = io.LimitedReader{R: body, N: readLimit}
	b.multipart.Reset(&b.limited, boundary)

	rejection := parseParts(b, &b.multipart, limits)

	readBytes := readLimit - b.limited.N
	// don't keep the body alive in the pool
	b.multipart.Reset(nil, "")
	b.limited.R = nil
	if limits.postMaxSize > 0 && readBytes > limits.postMaxSize {
		// the cut body is malformed too, the size is what matters
		rejection = tooLarge("Actual POST length does not match Content-Length, and exceeds %d bytes", limits.postMaxSize)
	}

	return readBytes, rejection
}

func parseParts(b *formBuffers, reader *multipart.Reader, limits formLimits) *formRejection {
	bodyParts := limits.maxMultipartBodyParts
	if bodyParts < 0 {
		bodyParts = limits.maxInputVars + limits.maxFileUploads
	}

	var (
		inputVars   int64
		fileUploads int64
		maxFileSize int64
	)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return malformed("Invalid multipart/form-data POST data: %s", err)
		}

		if bodyParts--; bodyParts < 0 {
			return tooLarge("Multipart body parts limit exceeded %d. To increase the limit change max_multipart_body_parts in php.ini.", limits.maxMultipartBodyParts)
		}

		name, filename, hasFilename, ok := part.FormData()
		if !ok {
			return malformed("File Upload Mime headers garbled")
		}

		if !hasFilename {
			start := len(b.data)
			if b.data, err = appendRead(b.data, part); err != nil {
				return malformed("Invalid multipart/form-data POST data: %s", err)
			}
			value := b.data[start:len(b.data):len(b.data)]

			if inputVars++; inputVars > limits.maxInputVars {
				return tooLarge("Input variables exceeded %d. To increase the limit change max_input_vars in php.ini.", limits.maxInputVars)
			}

			start = len(b.data)
			b.data = append(b.data, name...)
			b.entries = append(b.entries, formEntry{kind: C.FRANKENPHP_FORM_POST_STRING, name: b.data[start:len(b.data):len(b.data)], value: value})

			if bytes.EqualFold(name, []byte("MAX_FILE_SIZE")) {
				maxFileSize, _ = strconv.ParseInt(strings.TrimSpace(string(value)), 10, 64)
			}

			continue
		}

		// never repair potentially malicious input
		if !limits.fileUploads || !hasBalancedBrackets(string(name)) {
			continue
		}

		var (
			errorCode int64
			tmpName   string
			size      int64
		)

		// like PHP, empty file inputs don't count as uploads
		if len(filename) == 0 {
			errorCode = uploadErrNoFile
		} else {
			if fileUploads++; fileUploads > limits.maxFileUploads {
				return tooLarge("Maximum number of allowable file uploads has been exceeded")
			}

			tmpName, size, errorCode, err = saveUpload(part, limits, maxFileSize)
			if err != nil {
				return malformed("Invalid multipart/form-data POST data: %s", err)
			}
			if errorCode == uploadErrNoTmpDir {
				b.entries = append(b.entries, warning("File upload error - unable to create a temporary file"))
			}
		}

		mimeType := ""
		if errorCode == 0 {
			b.uploaded = append(b.uploaded, formEntry{kind: C.FRANKENPHP_FORM_UPLOADED_FILE, name: b.bytes(tmpName)})
			mimeType, _, _ = strings.Cut(string(part.ContentType()), ";")
		}

		b.fileEntries(string(name), string(filename), mimeType, tmpName, errorCode, size)
	}
}

// appendRead appends all of r to dst
func appendRead(dst []byte, r io.Reader) ([]byte, error) {
	for {
		if len(dst) == cap(dst) {
			dst = append(dst, 0)[:len(dst)]
		}

		n, err := r.Read(dst[len(dst):cap(dst)])
		dst = dst[:len(dst)+n]
		if err == io.EOF {
			return dst, nil
		}
		if err != nil {
			return dst, err
		}
	}
}

// removeUploads deletes the files of a body that won't be registered
func removeUploads(entries []formEntry) {
	for _, e := range entries {
		if e.kind == C.FRANKENPHP_FORM_UPLOADED_FILE {
			_ = os.Remove(string(e.name))
		}
	}
}

// multipart.Reader hands out at most 4 KiB per read: write bigger chunks
var uploadBufferPool = sync.Pool{New: func() any {
	b := make([]byte, 64<<10)

	return &b
}}

// saveUpload copies a file part to a new temporary file, the file is removed on error
func saveUpload(part io.Reader, limits formLimits, maxFileSize int64) (tmpName string, size int64, errorCode int64, err error) {
	f, err := os.CreateTemp(limits.tmpDir, "php")
	if err != nil {
		// drain the part: only reading errors must reject the body
		_, err = io.Copy(io.Discard, part)

		return "", 0, uploadErrNoTmpDir, err
	}

	bufp := uploadBufferPool.Get().(*[]byte)
	defer uploadBufferPool.Put(bufp)
	buf := *bufp

	filled := 0
	for {
		n, readErr := part.Read(buf[filled:])
		filled += n

		if n > 0 && errorCode == 0 {
			switch {
			case limits.uploadMaxFilesize > 0 && size+int64(n) > limits.uploadMaxFilesize:
				errorCode = uploadErrIniSize
			case maxFileSize != 0 && size+int64(n) > maxFileSize:
				errorCode = uploadErrFormSize
			default:
				size += int64(n)
			}
		}

		if filled == len(buf) || readErr != nil {
			if errorCode == 0 && filled > 0 {
				if _, err := f.Write(buf[:filled]); err != nil {
					errorCode = uploadErrCantWrite
				}
			}
			filled = 0
		}

		if readErr != nil {
			if readErr != io.EOF {
				err = readErr
			}

			break
		}
	}

	if closeErr := f.Close(); closeErr != nil && errorCode == 0 {
		errorCode = uploadErrCantWrite
	}

	if errorCode != 0 || err != nil {
		_ = os.Remove(f.Name())

		return "", 0, errorCode, err
	}

	return f.Name(), size, 0, nil
}

// fileEntries appends the $_FILES variables of an upload, "a[b]" registers "a[name][b]".
// Like rfc1867.c, the names registered for a file can't be overwritten by the next ones.
func (b *formBuffers) fileEntries(name, filename, mimeType, tmpName string, errorCode, size int64) {
	prefix, suffix := name, ""
	if i := strings.IndexByte(name, '['); i >= 0 && strings.HasSuffix(name, "]") {
		prefix, suffix = name[:i], name[i:]
	}

	key := func(k string) []byte {
		return b.bytes(prefix, "[", k, "]", suffix)
	}

	add := func(e formEntry, override bool) {
		b.normalized = appendNormalizedVariable(b.normalized[:0], e.name)
		if _, ok := b.protected[string(b.normalized)]; override || !ok {
			b.entries = append(b.entries, e)
		}
	}

	protect := func(v []byte) {
		b.normalized = appendNormalizedVariable(b.normalized[:0], v)
		b.protected[string(b.normalized)] = struct{}{}
	}

	// RFC 7578: don't use directory path information, full_path keeps it for folder uploads
	basename := filename
	if i := strings.LastIndexAny(filename, `/\`); i >= 0 {
		basename = filename[i+1:]
	}

	add(formEntry{kind: C.FRANKENPHP_FORM_FILES_STRING, name: key("name"), value: b.bytes(basename)}, false)
	add(formEntry{kind: C.FRANKENPHP_FORM_FILES_STRING, name: key("full_path"), value: b.bytes(filename)}, false)
	add(formEntry{kind: C.FRANKENPHP_FORM_FILES_STRING, name: key("type"), value: b.bytes(mimeType)}, false)

	tmpNameKey := key("tmp_name")
	protect(b.bytes(name))
	protect(tmpNameKey)

	add(formEntry{kind: C.FRANKENPHP_FORM_FILES_STRING, name: tmpNameKey, value: b.bytes(tmpName)}, true)
	add(formEntry{kind: C.FRANKENPHP_FORM_FILES_LONG, name: key("error"), number: errorCode}, false)
	add(formEntry{kind: C.FRANKENPHP_FORM_FILES_LONG, name: key("size"), number: size}, false)
}

// appendNormalizedVariable appends v normalized like normalize_protected_variable() of rfc1867.c
func appendNormalizedVariable(dst, v []byte) []byte {
	v = bytes.TrimLeft(v, " ")

	i := 0
	for ; i < len(v) && v[i] != '['; i++ {
		if v[i] == ' ' || v[i] == '.' {
			dst = append(dst, '_')
		} else {
			dst = append(dst, v[i])
		}
	}
	if i == len(v) {
		return dst
	}

	// keep each index up to its "]", without leading whitespace, and drop what follows the last one
	dst = append(dst, '[')
	rest := v[i+1:]
	for {
		rest = bytes.TrimLeft(rest, " \r\n\t")
		end := bytes.IndexByte(rest, ']') + 1
		if end == 0 {
			end = len(rest)
		}
		dst = append(dst, rest[:end]...)
		rest = rest[end:]

		if len(rest) == 0 || rest[0] != '[' {
			return dst
		}
		dst = append(dst, '[')
		rest = rest[1:]
	}
}

func hasBalancedBrackets(name string) bool {
	depth := 0
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '[':
			depth++
		case ']':
			depth--
			if i+1 < len(name) && name[i+1] != '[' {
				return false
			}
		}

		if depth < 0 {
			return false
		}
	}

	return depth == 0
}

// parseURLEncoded decodes an application/x-www-form-urlencoded body like PHP's add_post_vars() into dst,
// a NUL byte after each name, and passes the offsets of each field in dst to emit.
func parseURLEncoded(body, dst []byte, maxInputVars int64, emit func(nameStart, valueStart, valueEnd int)) ([]byte, *formRejection) {
	var count int64
	for len(body) > 0 {
		var pair []byte
		pair, body, _ = bytes.Cut(body, []byte{'&'})

		if count++; count > maxInputVars {
			return dst, tooLarge("Input variables exceeded %d. To increase the limit change max_input_vars in php.ini.", maxInputVars)
		}

		key, value, _ := bytes.Cut(pair, []byte{'='})

		nameStart := len(dst)
		dst = append(urlDecode(dst, key), 0)
		valueStart := len(dst)
		dst = urlDecode(dst, value)

		emit(nameStart, valueStart, len(dst))
	}

	return dst, nil
}

// urlDecode appends s to dst like php_url_decode(): invalid escapes are kept as is
func urlDecode(dst, s []byte) []byte {
	for {
		i := 0
		for i < len(s) && s[i] != '%' && s[i] != '+' {
			i++
		}
		dst = append(dst, s[:i]...)
		if i == len(s) {
			return dst
		}

		switch {
		case s[i] == '+':
			dst = append(dst, ' ')
			s = s[i+1:]
		case i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]):
			dst = append(dst, unhex(s[i+1])<<4|unhex(s[i+2]))
			s = s[i+3:]
		default:
			dst = append(dst, '%')
			s = s[i+1:]
		}
	}
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c <= '9':
		return c - '0'
	case c <= 'F':
		return c - 'A' + 10
	default:
		return c - 'a' + 10
	}
}

// toCFormResult copies groups of entries, in order, into a single malloc'ed block; names are NUL-terminated
func toCFormResult(groups ...[]formEntry) C.frankenphp_form_result {
	count, size := 0, 0
	for _, entries := range groups {
		count += len(entries)
		for _, e := range entries {
			size += len(e.name) + 1 + len(e.value)
		}
	}
	if count == 0 {
		return C.frankenphp_form_result{}
	}

	entriesSize := count * int(unsafe.Sizeof(C.frankenphp_form_entry{}))
	size += entriesSize

	block := C.malloc(C.size_t(size))
	cEntries := unsafe.Slice((*C.frankenphp_form_entry)(block), count)
	dataPtr := unsafe.Add(block, entriesSize)
	data := unsafe.Slice((*byte)(dataPtr), size-entriesSize)

	offset, i := 0, 0
	for _, entries := range groups {
		for _, e := range entries {
			name := unsafe.Add(dataPtr, offset)
			offset += copy(data[offset:], e.name)
			data[offset] = 0
			offset++

			value := unsafe.Add(dataPtr, offset)
			offset += copy(data[offset:], e.value)

			cEntries[i] = C.frankenphp_form_entry{
				kind:      e.kind,
				name:      (*C.char)(name),
				value:     (*C.char)(value),
				value_len: C.size_t(len(e.value)),
				number:    C.zend_long(e.number),
			}
			i++
		}
	}

	return C.frankenphp_form_result{entries: (*C.frankenphp_form_entry)(block), count: C.size_t(count)}
}

// settleForm answers 400 or 413 for a rejected body, or throws from request_parse_body()
func settleForm(fc *frankenPHPContext, rejection *formRejection, reject bool, groups ...[]formEntry) C.frankenphp_form_result {
	if rejection == nil {
		return toCFormResult(groups...)
	}

	for _, entries := range groups {
		removeUploads(entries)
	}

	if fc.logger.Enabled(fc.ctx, slog.LevelDebug) {
		fc.logger.LogAttrs(fc.ctx, slog.LevelDebug, "request body rejected", slog.Int("status", rejection.rejected.status), slog.String("reason", rejection.detail))
	}

	if !reject {
		return toCFormResult([]formEntry{warning("%s", rejection.detail)})
	}

	if !fc.headersWritten {
		fc.reject(rejection.rejected)
	}

	return C.frankenphp_form_result{}
}

//export go_parse_multipart
func go_parse_multipart(threadIndex C.uintptr_t, contentType *C.char, cLimits *C.frankenphp_form_limits) C.frankenphp_form_result {
	fc := phpThreads[threadIndex].handler.frankenPHPContext()

	// net/http forbids reading the body after the handler returns
	if fc.responseWriter == nil || fc.isDone {
		return C.frankenphp_form_result{}
	}

	limits := formLimits{
		postMaxSize:           int64(cLimits.post_max_size),
		maxInputVars:          int64(cLimits.max_input_vars),
		maxFileUploads:        int64(cLimits.max_file_uploads),
		maxMultipartBodyParts: int64(cLimits.max_multipart_body_parts),
		uploadMaxFilesize:     int64(cLimits.upload_max_filesize),
		fileUploads:           bool(cLimits.file_uploads),
		tmpDir:                C.GoString(cLimits.tmp_dir),
	}

	b := formBuffersPool.Get().(*formBuffers)
	defer b.release()

	body := newBodyReader(fc)
	readBytes, rejection := parseMultipart(b, body, C.GoString(contentType), fc.request.ContentLength, limits)
	tooLarge := body.close()

	// a body cut by a stalled client or by request_body limits isn't malformed
	if rejection != nil && rejection.rejected.status == http.StatusBadRequest && body.failed() {
		removeUploads(b.uploaded)
		b.uploaded, b.entries, rejection = b.uploaded[:0], b.entries[:0], nil
	}

	result := settleForm(fc, rejection, bool(cLimits.reject), b.uploaded, b.entries)
	result.read_bytes = C.size_t(readBytes)
	result.too_large = C.bool(tooLarge)

	return result
}

//export go_reject_body_too_large
func go_reject_body_too_large(threadIndex C.uintptr_t) {
	phpThreads[threadIndex].handler.frankenPHPContext().rejectBodyTooLarge()
}

//export go_parse_urlencoded
func go_parse_urlencoded(threadIndex C.uintptr_t, cBody *C.char, length C.size_t, maxInputVars C.zend_long, reject C.bool) C.frankenphp_form_result {
	body := unsafe.Slice((*byte)(unsafe.Pointer(cBody)), length)

	// decode in the block handed to C: decoding never grows the data, plus a NUL per name
	maxFields := bytes.Count(body, []byte{'&'}) + 1
	if maxInputVars >= 0 && int64(maxFields) > int64(maxInputVars) {
		// parseURLEncoded rejects the body before emitting more fields
		maxFields = int(maxInputVars)
	}
	entriesSize := maxFields * int(unsafe.Sizeof(C.frankenphp_form_entry{}))
	block := C.malloc(C.size_t(entriesSize + len(body) + maxFields))
	entries := unsafe.Slice((*C.frankenphp_form_entry)(block), maxFields)
	dataPtr := unsafe.Add(block, entriesSize)
	data := unsafe.Slice((*byte)(dataPtr), len(body)+maxFields)[:0]

	count := 0
	data, rejection := parseURLEncoded(body, data, int64(maxInputVars), func(nameStart, valueStart, valueEnd int) {
		entries[count] = C.frankenphp_form_entry{
			kind:      C.FRANKENPHP_FORM_POST_STRING,
			name:      (*C.char)(unsafe.Add(dataPtr, nameStart)),
			value:     (*C.char)(unsafe.Add(dataPtr, valueStart)),
			value_len: C.size_t(valueEnd - valueStart),
		}
		count++
	})

	if cap(data) > 0 && unsafe.Pointer(unsafe.SliceData(data)) != dataPtr {
		panic("urlencoded data outgrew its buffer")
	}

	if rejection != nil || count == 0 {
		C.free(block)

		return settleForm(phpThreads[threadIndex].handler.frankenPHPContext(), rejection, bool(reject))
	}

	return C.frankenphp_form_result{entries: (*C.frankenphp_form_entry)(block), count: C.size_t(count)}
}
