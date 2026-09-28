package dict

import (
	"bytes"
	_ "embed"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

//go:embed stardict.csv
var dataCSV []byte

//go:embed cedict.csv
var cedictCSV []byte

//go:embed us_phonetic.tsv
var usPhoneticTSV []byte

// dataStart / cedictStart are the byte offsets of the first data row (right
// after the header line) of each CSV. Both CSVs are sorted by lowercased
// word, so lookups can binary search directly into the embedded bytes.
var (
	dataStart   = bytes.IndexByte(dataCSV, '\n') + 1
	cedictStart = bytes.IndexByte(cedictCSV, '\n') + 1
)

// Entry mirrors a single row of ECDICT's stardict.csv.
// Column layout is the same as stardict.py's COLUMN_SIZE = 13.
type Entry struct {
	Word        string
	Phonetic    string
	Definition  string
	Translation string
	Pos         string
	Collins     int
	Oxford      int
	Tag         string
	BNC         int
	Frq         int
	Exchange    string
	Detail      string
	Audio       string

	sw string // stripword(Word), computed lazily for strip-match
}

var (
	stripOnce sync.Once
	stripIdx  []*Entry // sorted by (sw, lowercased word)
)

// stripword keeps only alphanumeric characters and lowercases the result,
// mirroring stardict.py's stripword.
func stripword(word string) string {
	var b strings.Builder
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// decode unescapes ECDICT's CSV escapes: \\n, \\r and \\\\.
func decode(text string) string {
	if text == "" || !strings.Contains(text, "\\") {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\\' && i+1 < len(text) {
			switch text[i+1] {
			case '\\':
				b.WriteByte('\\')
				i++
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			default:
				b.WriteByte('\\')
				b.WriteByte(text[i+1])
				i++
			}
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func readint(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// splitCSVLine splits one CSV record into fields. ECDICT records never
// contain embedded newlines (they store them as the two-character escape
// "\n"), so each record is exactly one line.
func splitCSVLine(line string, out []string) []string {
	out = out[:0]
	i := 0
	for i < len(line) {
		var field string
		if line[i] == '"' {
			i++
			var b strings.Builder
			for i < len(line) {
				c := line[i]
				if c == '"' {
					if i+1 < len(line) && line[i+1] == '"' {
						b.WriteByte('"')
						i += 2
						continue
					}
					i++ // closing quote
					break
				}
				b.WriteByte(c)
				i++
			}
			field = b.String()
			if i < len(line) && line[i] == ',' {
				i++
			}
		} else {
			start := i
			if j := strings.IndexByte(line[start:], ','); j >= 0 {
				field = line[start : start+j]
				i = start + j + 1
			} else {
				field = line[start:]
				i = len(line)
			}
		}
		out = append(out, field)
	}
	// A trailing comma means the last field is empty.
	if strings.HasSuffix(line, ",") {
		out = append(out, "")
	}
	return out
}

// findLine returns the line containing byte offset p in data. It returns the
// line start offset, the offset just past the line's newline, and the line
// bytes with the CRLF/LF ending stripped.
func findLine(data []byte, start int, p int) (lineStart, next int, line []byte) {
	lineStart = p
	for lineStart > start && data[lineStart-1] != '\n' {
		lineStart--
	}
	end := lineStart
	for end < len(data) && data[end] != '\n' {
		end++
	}
	next = end
	if next < len(data) {
		next++ // skip '\n'
	}
	line = data[lineStart:end]
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return lineStart, next, line
}

// wordOfLine extracts the first CSV field (the word) from one record. Words
// containing a comma are quoted.
func wordOfLine(line []byte) string {
	if len(line) == 0 {
		return ""
	}
	if line[0] == '"' {
		if end := bytes.IndexByte(line[1:], '"'); end >= 0 {
			return string(line[1 : 1+end])
		}
		return ""
	}
	if i := bytes.IndexByte(line, ','); i >= 0 {
		return string(line[:i])
	}
	return string(line)
}

// parseEntry parses a single stardict.csv record (13 columns, without line
// ending) into an Entry.
func parseEntry(line []byte) *Entry {
	fields := splitCSVLine(string(line), nil)
	if len(fields) < 13 {
		return nil
	}
	e := &Entry{
		Word:        decode(fields[0]),
		Phonetic:    decode(fields[1]),
		Definition:  decode(fields[2]),
		Translation: decode(fields[3]),
		Pos:         decode(fields[4]),
		Collins:     readint(fields[5]),
		Oxford:      readint(fields[6]),
		Tag:         decode(fields[7]),
		BNC:         readint(fields[8]),
		Frq:         readint(fields[9]),
		Exchange:    decode(fields[10]),
		Detail:      decode(fields[11]),
		Audio:       decode(fields[12]),
	}
	if e.Word == "" {
		return nil
	}
	return e
}

// parseCedictEntry parses a single cedict.csv record (word,pinyin,translation)
// into an Entry, mapping Chinese -> English. Numeric-tone pinyin is converted
// to tone-marked Unicode (de2 guo2 -> dé guó).
func parseCedictEntry(line []byte) *Entry {
	fields := splitCSVLine(string(line), nil)
	if len(fields) < 3 {
		return nil
	}
	e := &Entry{
		Word:        decode(fields[0]),
		Phonetic:    pinyinToToneMarks(decode(fields[1])),
		Translation: decode(fields[2]),
	}
	if e.Word == "" {
		return nil
	}
	return e
}

// pinyinToToneMarks converts CC-CEDICT's numeric-tone pinyin ("de2 guo2")
// into tone-marked Unicode ("dé guó"). Neutral tone (5) and unnumbered
// syllables are left unmarked.
func pinyinToToneMarks(py string) string {
	if py == "" {
		return py
	}
	fields := strings.Fields(py)
	for i, syl := range fields {
		fields[i] = markPinyinSyllable(syl)
	}
	return strings.Join(fields, " ")
}

// markPinyinSyllable applies a trailing tone digit (1-4) to the correct
// vowel of a pinyin syllable, following the standard placement rule: mark
// "a" or "e" if present, then "ou", otherwise the last vowel.
func markPinyinSyllable(syl string) string {
	tone := 0
	if n := len(syl); n > 0 && syl[n-1] >= '1' && syl[n-1] <= '5' {
		tone = int(syl[n-1] - '0')
		syl = syl[:n-1]
	}
	// CC-CEDICT spells ü as "u:".
	syl = strings.ReplaceAll(syl, "u:", "ü")
	if tone < 1 || tone > 4 {
		return syl
	}

	runes := []rune(syl)
	idx := -1
	for i, r := range runes {
		if r == 'a' || r == 'e' {
			idx = i
			break
		}
	}
	if idx < 0 {
		for i := 0; i < len(runes)-1; i++ {
			if runes[i] == 'o' && runes[i+1] == 'u' {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		for i := len(runes) - 1; i >= 0; i-- {
			if r := runes[i]; r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u' || r == 'ü' {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		return syl
	}
	marks := toneMarks[runes[idx]]
	if len(marks) == 0 {
		return syl
	}
	runes[idx] = marks[tone-1]
	return string(runes)
}

// toneMarks maps each pinyin vowel to its four tone-marked forms.
var toneMarks = map[rune][4]rune{
	'a': {'ā', 'á', 'ǎ', 'à'},
	'e': {'ē', 'é', 'ě', 'è'},
	'i': {'ī', 'í', 'ǐ', 'ì'},
	'o': {'ō', 'ó', 'ǒ', 'ò'},
	'u': {'ū', 'ú', 'ǔ', 'ù'},
	'ü': {'ǖ', 'ǘ', 'ǚ', 'ǜ'},
}

// search finds the byte offset of the first record in data whose lowercased
// word is >= key. It returns len(data) if no such record exists.
func search(data []byte, start int, key string) int {
	lo, hi := start, len(data)
	for lo < hi {
		mid := lo + (hi-lo)/2
		ls, next, line := findLine(data, start, mid)
		if strings.ToLower(wordOfLine(line)) < key {
			lo = next
		} else {
			hi = ls
		}
	}
	return lo
}

// wordOfTSV extracts the word (everything before the first tab) from one line
// of us_phonetic.tsv.
func wordOfTSV(line []byte) string {
	if i := bytes.IndexByte(line, '\t'); i >= 0 {
		return string(line[:i])
	}
	return ""
}

// ipaOfTSV extracts the IPA transcription (everything after the first tab)
// from one line of us_phonetic.tsv.
func ipaOfTSV(line []byte) string {
	if i := bytes.IndexByte(line, '\t'); i >= 0 {
		return string(line[i+1:])
	}
	return ""
}

// searchTSV is search over a headerless TSV table sorted by lowercased word.
func searchTSV(table []byte, key string) int {
	lo, hi := 0, len(table)
	for lo < hi {
		mid := lo + (hi-lo)/2
		ls, next, line := findLine(table, 0, mid)
		if strings.ToLower(wordOfTSV(line)) < key {
			lo = next
		} else {
			hi = ls
		}
	}
	return lo
}

// USPhonetic returns the American English IPA transcription for word from
// the embedded us_phonetic.tsv table, or "" if the word has no entry.
func USPhonetic(word string) string {
	key := strings.ToLower(word)
	pos := searchTSV(usPhoneticTSV, key)
	if pos >= len(usPhoneticTSV) {
		return ""
	}
	_, _, line := findLine(usPhoneticTSV, 0, pos)
	if strings.ToLower(wordOfTSV(line)) != key {
		return ""
	}
	return ipaOfTSV(line)
}

// lookup finds word in data (sorted by lowercased word) and parses the
// matching record with parseFn.
func lookup(data []byte, start int, word string, parseFn func([]byte) *Entry) *Entry {
	key := strings.ToLower(word)
	pos := search(data, start, key)
	if pos >= len(data) {
		return nil
	}
	_, _, line := findLine(data, start, pos)
	if strings.ToLower(wordOfLine(line)) != key {
		return nil
	}
	return parseFn(line)
}

// Lookup returns the English->Chinese dictionary entry for the given word
// (case-insensitive), or nil if not found. It mirrors stardict.py
// DictCsv.query with a string key.
func Lookup(word string) *Entry {
	return lookup(dataCSV, dataStart, word, parseEntry)
}

// LookupZH returns the Chinese->English dictionary entry (from CC-CEDICT) for
// the given simplified Chinese word, or nil if not found.
func LookupZH(word string) *Entry {
	return lookup(cedictCSV, cedictStart, word, parseCedictEntry)
}

// Match returns up to count entries starting at the given word, like
// stardict.py DictCsv.match. When strip is true, words are matched on their
// stripped (alphanumeric-only, lowercased) form.
func Match(word string, count int, strip bool) []*Entry {
	if count <= 0 {
		return nil
	}
	if strip {
		return matchStrip(word, count)
	}
	pos := search(dataCSV, dataStart, strings.ToLower(word))
	result := make([]*Entry, 0, count)
	for p := pos; p < len(dataCSV) && len(result) < count; {
		_, next, line := findLine(dataCSV, dataStart, p)
		if e := parseEntry(line); e != nil {
			result = append(result, e)
		}
		p = next
	}
	return result
}

// matchStrip builds (once) and searches an index ordered by stripword, then
// by lowercased word, mirroring stardict.py DictCsv.match(strip=True).
func matchStrip(word string, count int) []*Entry {
	stripOnce.Do(func() {
		stripIdx = make([]*Entry, 0, Count())
		for p := dataStart; p < len(dataCSV); {
			_, next, line := findLine(dataCSV, dataStart, p)
			if e := parseEntry(line); e != nil {
				e.sw = stripword(e.Word)
				stripIdx = append(stripIdx, e)
			}
			p = next
		}
		sort.Slice(stripIdx, func(i, j int) bool {
			if stripIdx[i].sw != stripIdx[j].sw {
				return stripIdx[i].sw < stripIdx[j].sw
			}
			return strings.ToLower(stripIdx[i].Word) < strings.ToLower(stripIdx[j].Word)
		})
	})
	key := stripword(word)
	i := sort.Search(len(stripIdx), func(i int) bool { return stripIdx[i].sw >= key })
	end := i + count
	if end > len(stripIdx) {
		end = len(stripIdx)
	}
	return stripIdx[i:end]
}

// Count returns the number of dictionary entries.
func Count() int {
	return bytes.Count(dataCSV, []byte{'\n'}) - 1 // minus header
}

// Has reports whether word is in the dictionary.
func Has(word string) bool {
	return Lookup(word) != nil
}
