package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/major1201/translate-cli/dict"
	"github.com/major1201/translate-cli/translate"
)

func main() {
	os.Exit(run())
}

// usage prints the full help text. It is used both for the no-argument case
// and by the flag package for -h/--help, so the two always match.
func usage() {
	fmt.Fprintln(os.Stderr, "Usage: tx [options] <text>")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Environment:")
	fmt.Fprintln(os.Stderr, "  TX_LLM_API_KEY  API key for LLM translation (or OPENAI_API_KEY)")
	fmt.Fprintln(os.Stderr, "  TX_LLM_BASE_URL Base URL for LLM API (or OPENAI_BASE_URL)")
	fmt.Fprintln(os.Stderr, "  TX_LLM_MODEL    Model name (or OPENAI_MODEL, default: gpt-4o-mini)")
}

func run() int {
	sourceLang := flag.String("s", "en", "source language (e.g., en, zh, ja)")
	targetLang := flag.String("t", "zh", "target language (e.g., zh, en, ja)")
	dictOnly := flag.Bool("d", false, "dictionary lookup only, skip LLM translation")
	llmOnly := flag.Bool("l", false, "LLM translation only, skip local dictionary")
	flag.Usage = usage
	flag.Parse()

	// Track which language flags were explicitly set, so auto-detection only
	// fills in what the user left unspecified.
	explicit := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicit[f.Name] = true
	})

	if *dictOnly && *llmOnly {
		fmt.Fprintln(os.Stderr, "-d and -l cannot be used together")
		return 1
	}

	text := strings.Join(flag.Args(), " ")
	if text == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Reading stdin: %v\n", err)
			return 1
		}
		text = string(data)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		usage()
		return 1
	}

	// Auto-detect the source language and fill in whichever language flags the
	// user left unspecified. Chinese defaults to English output; everything
	// else defaults to Chinese output.
	detected := detectLang(text)
	zhText := detected == "zh"
	if !explicit["s"] {
		*sourceLang = detected
	}
	if !explicit["t"] && *sourceLang == "zh" {
		*targetLang = "en"
	}

	// The built-in dictionaries translate en->zh and zh->en. Only use them
	// when their output language matches the requested target; otherwise fall
	// through to the LLM (e.g. -t ja). Dictionary-only mode (-d) always uses
	// the dictionary.
	dictUseful := (zhText && *targetLang == "en") || (!zhText && *targetLang == "zh")

	var entry *dict.Entry
	var found []*dict.Entry
	if !*llmOnly && (*dictOnly || dictUseful) {
		// 1. Try dictionary lookup first (direction follows the detected text).
		if zhText {
			entry = dict.LookupZH(text)
		} else {
			entry = dict.Lookup(text)
		}
		if entry != nil {
			printDictEntry(entry, !zhText)
			return 0
		}

		// Also try each word individually if it's a phrase.
		words := strings.Fields(text)
		if len(words) > 1 {
			for _, w := range words {
				var e *dict.Entry
				if zhText {
					e = dict.LookupZH(w)
				} else {
					e = dict.Lookup(w)
				}
				if e != nil {
					found = append(found, e)
				}
			}
			if len(found) > 0 {
				fmt.Println("--- Dictionary matches for individual words ---")
				for _, e := range found {
					printDictEntry(e, !zhText)
				}
				fmt.Println()
			}
		}
	}

	// 2. If dictionary-only mode, stop here
	if *dictOnly {
		if entry == nil && len(found) == 0 {
			fmt.Println("Not found in dictionary.")
		}
		return 0
	}

	// 3. Fall back to LLM translation (streamed to stdout)
	translator := translate.New(nil)
	fmt.Printf("🌐 LLM Translation (%s → %s):\n", *sourceLang, *targetLang)
	if _, err := translator.Translate(text, *sourceLang, *targetLang, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Translation error: %v\n", err)
		return 1
	}
	fmt.Println()
	return 0
}

// exchangeLabels maps the ECDICT exchange field keys to their Chinese
// labels, following stardict.py's DictHelper._exchanges.
var exchangeLabels = map[string]string{
	"p": "过去式",
	"d": "过去分词",
	"i": "现在分词",
	"3": "第三人称单数",
	"r": "比较级",
	"t": "最高级",
	"s": "复数",
	"0": "原型",
	"1": "类别",
}

func printDictEntry(e *dict.Entry, enPhon bool) {
	fmt.Printf("📖 %s", e.Word)
	if enPhon {
		switch us := dict.USPhonetic(e.Word); {
		case e.Phonetic != "" && us != "":
			fmt.Printf("  BrE /%s/, AmE /%s/", normalizePhonetic(e.Phonetic), us)
		case us != "":
			fmt.Printf("  AmE /%s/", us)
		case e.Phonetic != "":
			fmt.Printf("  BrE /%s/", normalizePhonetic(e.Phonetic))
		}
	} else if e.Phonetic != "" {
		fmt.Printf("  /%s/", e.Phonetic)
	}
	fmt.Println()
	if e.Translation != "" {
		for line := range strings.SplitSeq(e.Translation, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fmt.Printf("   %s\n", line)
			}
		}
	}
	// The exchange field looks like "p:kissed/i:kissing/s:kisses/...";
	// print each variant on its own line with a Chinese label.
	for part := range strings.SplitSeq(e.Exchange, "/") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, ":")
		if value == "" {
			continue
		}
		label := exchangeLabels[key]
		if label == "" {
			label = key
		}
		fmt.Printf("   %s：%s\n", label, value)
	}
}

// normalizePhonetic rewrites the non-standard glyphs ECDICT uses in its
// British transcriptions to regular IPA so they read like the US table:
// the ASCII apostrophe stress mark becomes U+02C8 (ˈ) and the Cyrillic
// schwa U+04D9 (ә) becomes the IPA schwa U+0259 (ə).
func normalizePhonetic(ph string) string {
	return strings.NewReplacer(
		"ә", "ə",
		"'", "ˈ",
	).Replace(ph)
}

// detectLang guesses the source language of text from its Unicode script.
// Japanese is detected by the presence of kana, Korean by Hangul, etc.;
// Latin-script text defaults to English.
func detectLang(text string) string {
	var han, kana, hangul, cyrillic, arabic, thai int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			han++
		case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
			kana++
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case unicode.Is(unicode.Cyrillic, r):
			cyrillic++
		case unicode.Is(unicode.Arabic, r):
			arabic++
		case unicode.Is(unicode.Thai, r):
			thai++
		}
	}
	switch {
	case kana > 0:
		return "ja"
	case hangul > 0:
		return "ko"
	case cyrillic > 0:
		return "ru"
	case arabic > 0:
		return "ar"
	case thai > 0:
		return "th"
	case han > 0:
		return "zh"
	default:
		return "en"
	}
}
