#!/usr/bin/env python3
"""Generate Resources/us_phonetic.tsv from the ipa-dict en_US wordlist.

Usage: gen_us_phonetic.py <en_US.txt> <output.tsv>

The ipa-dict en_US data (MIT, derived from CMUdict via cmudict-ipa +
syllabify) consists of one "word<TAB>/ipa1/, /ipa2/" line per entry. This
writes a two-column tab-separated file "word<TAB>ipa" keeping only the first
pronunciation and stripping the surrounding slashes, sorted by lowercased
word. The phonemic symbols are normalized to a learner-friendly
(Merriam-Webster-like) AmE transcription: ɝ/ɚ → ər, ɫ → l, ɹ → r, ɡ → g.
The app loads the file once into an in-memory lookup table.
"""

import sys


def normalize(ipa):
    """Rewrite uncommon IPA glyphs to a learner-friendly AmE notation."""
    for src, dst in (
        ("ɝ", "ər"),
        ("ɚ", "ər"),
        ("ɫ", "l"),
        ("ɹ", "r"),
        ("ɡ", "g"),
    ):
        ipa = ipa.replace(src, dst)
    return ipa


def convert(src_path, dst_path):
    entries = {}
    with open(src_path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.rstrip("\n")
            if not line:
                continue
            word, _, ipas = line.partition("\t")
            if not ipas:
                continue
            # Keep only the first pronunciation: /ˈeɪ/, /ə/ -> /ˈeɪ/
            first = ipas.split(",", 1)[0].strip()
            if first.startswith("/") and first.endswith("/"):
                first = first[1:-1]
            if not first:
                continue
            entries[word] = normalize(first)

    with open(dst_path, "w", encoding="utf-8") as f:
        for word in sorted(entries, key=str.lower):
            f.write(f"{word}\t{entries[word]}\n")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("usage: gen_us_phonetic.py <en_US.txt> <output.tsv>", file=sys.stderr)
        sys.exit(2)
    convert(sys.argv[1], sys.argv[2])
