// cgi_unicode_extra_regression_test.go — PR-ready regression suite.
//
// Regression vectors pinning the splitPos() bug class fixed by
// GHSA-g966-83w7-6w38 (CVE-2026-24895) and GHSA-3g8v-8r37-cgjm
// (CVE-2026-45062). All tests must pass on current main (v1.13.x+):
// this is NOT a bug report against a supported version — it is
// permanent coverage so the whole class cannot regress:
//
//   1. bytes >= utf8.RuneSelf can never satisfy an ASCII-only split
//      marker, and a bogus split must never slice mid-UTF-8-rune
//      (residual edge case that existed in the v1.11.2–v1.12.3 fix,
//      commit 04fdc0c, and was resolved by the v1.13.0 rewrite);
//   2. U+023E (Ⱦ → ⱦ) is the only codepoint besides U+023A (Ⱥ → ⱥ)
//      whose simple lowercase mapping GROWS its UTF-8 length (2B → 3B);
//   3. the "shrink" direction: 23 codepoints (incl. U+1E9E ẞ 3B→2B and
//      U+212A K 3B→1B) whose lowercase is SHORTER — none were covered
//      by the original fix's tests;
//   4. the segment-boundary rule for SCRIPT_NAME/PATH_INFO splitting,
//      including the exact advisory PoC path.
//
// Place this file beside cgi_test.go (same package). The single call
// site lives in currentSplitPos(); adjust it if the internal helper
// signature differs in your tree.
package frankenphp

import (
	"testing"
	"unicode/utf8"
)

// currentSplitPos adapts this suite to the internal implementation.
// The second argument is the split-marker list used for PHP scripts.
func currentSplitPos(path string) int {
	return splitPos(path, []string{".php"})
}

// assertValidSplit enforces the invariants of a correct split:
//   - pos == -1 is always acceptable (no script detected);
//   - pos >= 0 must yield valid-UTF-8 SCRIPT_NAME and PATH_INFO;
//   - pos >= 0 must sit on a segment boundary (PATH_INFO empty or
//     starting with '/').
func assertValidSplit(t *testing.T, path string, want int) {
	t.Helper()
	got := currentSplitPos(path)
	if got != want {
		t.Errorf("splitPos(%q) = %d, want %d", path, got, want)
		return
	}
	if got < 0 {
		return
	}
	scriptName, pathInfo := path[:got], path[got:]
	if !utf8.ValidString(scriptName) {
		t.Errorf("splitPos(%q) = %d: SCRIPT_NAME %q is invalid UTF-8 (mid-rune split)", path, got, scriptName)
	}
	if !utf8.ValidString(pathInfo) {
		t.Errorf("splitPos(%q) = %d: PATH_INFO %q is invalid UTF-8 (mid-rune split)", path, got, pathInfo)
	}
	if pathInfo != "" && pathInfo[0] != '/' {
		t.Errorf("splitPos(%q) = %d: PATH_INFO %q does not start on a segment boundary", path, got, pathInfo)
	}
}

// TestSplitPosMultiByteNeverMatches pins the residual edge case of the
// v1.11.2 fix (04fdc0c): a bogus ASCII prefix window followed by a
// multi-byte byte must NOT produce a split at all — neither when the
// marker is absent from the path (v1.11.2–v1.12.3 returned 6 / 11 here,
// slicing mid-rune: scriptName "/x.ph\xC8", pathInfo "\xBAy") nor with
// raw lone continuation bytes. Expected result on current main: -1.
func TestSplitPosMultiByteNeverMatches(t *testing.T) {
	cases := []struct {
		path string
		note string
	}{
		{"/x.phȺy", "ASCII prefix \".ph\" + U+023A; marker \".php\" absent (was 6 on v1.11.2–v1.12.3)"},
		{"/upload.phȾnote", "ASCII prefix \".ph\" + U+023E; marker absent (was 11)"},
		{"/x.ph\xC8y", "raw lone byte 0xC8 (invalid UTF-8); marker absent (was 6)"},
		{"/x.ph\xC8", "raw lone byte 0xC8 at end of path"},
		{"/Ⱥ.phȾy", "marker window flanked by multi-byte bytes on both sides"},
	}
	for _, c := range cases {
		t.Run(c.note, func(t *testing.T) {
			assertValidSplit(t, c.path, -1)
		})
	}
}

// TestSplitPosOtherExpansionChar completes the expansion set. U+023A (Ⱥ → ⱥ)
// and U+023E (Ⱦ → ⱦ) are the ONLY two codepoints in the whole Unicode range
// whose simple lowercase mapping grows the UTF-8 byte length (2B → 3B) in Go
// (unicode.ToLower / strings.ToLower semantics). The original fix's tests
// covered Ⱥ and U+0130; Ⱦ was not covered.
func TestSplitPosOtherExpansionChar(t *testing.T) {
	// Advisory PoC path under current semantics: first ".php" (bytes 14–18)
	// is not on a segment boundary, so the split resolves to the last
	// occurrence (26 == len(path)) — the whole path becomes SCRIPT_NAME and
	// the lookup 404s instead of executing "ȺȺȺȺshell.php.txt".
	assertValidSplit(t, "/ȺȺȺȺshell.php.txt.php", 26)

	// Same shape with Ⱦ, the other expansion codepoint.
	assertValidSplit(t, "/ȾȾȾȾshell.php.txt.php", 26)

	// Expansion chars inside the script-name segment must not shift the
	// split: the marker bytes are ASCII and their position is byte-exact.
	// ("/ȺȾ" = 1 + 2 + 2 = 5 bytes + ".php" = 9 bytes total)
	assertValidSplit(t, "/ȺȾ.php", 9)
	assertValidSplit(t, "/ȺȾ.php/a", 9) // pathinfo preserved
}

// TestSplitPosShrinkingChars pins the OPPOSITE direction: codepoints whose
// simple lowercase mapping SHRINKS the UTF-8 length. Full scan (U+0080..
// U+10FFFF, Go simple-lowercase semantics) finds 23 of them: U+0130 (İ,
// 2B→1B), U+1E9E (ẞ, 3B→2B), U+2126 (Ω, 3B→2B), U+212A (K, 3B→1B),
// U+212B (Å, 3B→2B) and the Latin Extended-C/D compatibility block
// (U+2C62, U+2C64, U+2C6D..U+2C70, U+2C7E, U+2C7F, U+A78D, U+A7AA..U+A7AE,
// U+A7B0..U+A7B2, U+A7C5). Under the pre-fix strings.ToLower implementation
// the shorter lowercase shifted the byte index backwards: e.g. splitPos
// computed on "/k.php" (6B) applied to "/K.php" (8B) split at byte 6,
// executing a file named "K.p". Current implementation must split on the
// literal ASCII bytes.
func TestSplitPosShrinkingChars(t *testing.T) {
	// Plain end-of-path markers: split at the real ".php" (byte-exact).
	assertValidSplit(t, "/ẞ.php", 8)      // U+1E9E, 3B→2B (drift −1)
	assertValidSplit(t, "/\u212A.php", 8) // U+212A KELVIN SIGN, 3B→1B (drift −2)
	assertValidSplit(t, "/\u2126.php", 8) // U+2126 OHM SIGN, 3B→2B
	assertValidSplit(t, "/\u212B.php", 8) // U+212B ANGSTROM SIGN, 3B→2B
	assertValidSplit(t, "/İ.php", 7)      // U+0130, 2B→1B
	assertValidSplit(t, "/\u2C6F.php", 8) // U+2C6F Ɐ, 3B→2B (Latin Ext-C/D class)

	// Shrink char + non-segment-boundary marker: must refuse the split
	// instead of slicing mid-name (pre-fix code executed "ẞ.ph" here).
	assertValidSplit(t, "/ẞ.php.txt", -1)
	assertValidSplit(t, "/\u212A.php.txt", -1)
}

// TestSplitPosUnicodeSegmentBoundary pins the interaction between Unicode
// paths and the segment-boundary rule: the split is only valid where
// PATH_INFO is empty or starts with '/'. Includes the exact advisory PoC
// path (/ȺȺȺȺshell.php.txt.php → 26 under current semantics) and a
// mid-filename marker case.
func TestSplitPosUnicodeSegmentBoundary(t *testing.T) {
	// Pathinfo must keep working (also with Unicode in the script segment).
	assertValidSplit(t, "/index.php/foo", 10)
	assertValidSplit(t, "/ȺȺȺȺshell.php/info", 18)

	// Marker in the middle of a filename: not a segment boundary → no split.
	assertValidSplit(t, "/x.phpy", -1)
	assertValidSplit(t, "/ȺȺȺȺshell.php.txt", -1)

	// Control: plain script.
	assertValidSplit(t, "/ok.php", 7)
}