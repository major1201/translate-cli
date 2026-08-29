#!/usr/bin/env python3
"""Generate dict/cedict.csv from the CC-CEDICT MDBG export.

Usage: gen_cedict.py <cedict_ts_utf-8_mdbg.txt.gz> <output.csv>

Converts the CC-CEDICT text format into a 3-column CSV
(word,pinyin,translation) sorted by lowercased word. Duplicate simplified
words (from different traditional variants) are merged, keeping the first
reading and concatenating all definitions. Definitions are joined with the
literal two-character escape "\\n", matching the escape convention used by
the embedded stardict.csv and decoded by dict.decode.
"""

import csv
import gzip
import re
import sys

# traditional simplified [pinyin] /def1/def2/  (optionally two [pinyin] blocks
# when the traditional and simplified readings differ)
LINE_RE = re.compile(r"^(\S+)\s+(\S+)\s+\[([^\]]+)\](?:\s+\[([^\]]+)\])?\s*/(.*)/$")


def convert(src_path, dst_path):
    entries = {}  # simplified word -> [pinyin, [definitions...]]
    with gzip.open(src_path, "rt", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            m = LINE_RE.match(line)
            if not m:
                continue
            simp = m.group(2)
            # Prefer the simplified reading when two pinyin blocks exist.
            pinyin = (m.group(4) or m.group(3)).lower()
            defs = m.group(5).split("/")

            if simp in entries:
                existing = entries[simp][1]
                for d in defs:
                    if d not in existing:
                        existing.append(d)
            else:
                entries[simp] = [pinyin, defs]

    with open(dst_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f, lineterminator="\r\n")
        w.writerow(["word", "pinyin", "translation"])
        for word in sorted(entries, key=str.lower):
            pinyin, defs = entries[word]
            w.writerow([word, pinyin, "\\n".join(defs)])


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("usage: gen_cedict.py <cedict_ts.gz> <output.csv>", file=sys.stderr)
        sys.exit(2)
    convert(sys.argv[1], sys.argv[2])
