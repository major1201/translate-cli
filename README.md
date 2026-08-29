# translate-cli

A fast, offline-first command-line translation tool.

`translate-cli` (command: `tx`) translates text right in your terminal. It
tries the bundled offline dictionaries first (instant, no network, no API
key) and falls back to a streaming LLM translation when the dictionaries
don't cover the language pair.

## Features

- **Auto language detection** — the source language is inferred from the
  text's Unicode script, so `-s` is usually unnecessary. Detects Chinese,
  English, Japanese, Korean, Russian, Arabic, and Thai.
- **Offline dictionaries** — a 3.4-million-entry English→Chinese dictionary
  (ECDICT) and a ~121,000-entry Chinese→English dictionary (CC-CEDICT),
  queried by binary search with millisecond startup.
- **LLM translation** — any OpenAI-compatible Chat Completions API, streamed
  token by token.
- **Sensible defaults** — Chinese input is translated to English; everything
  else is translated to Chinese.

## Requirements

- Go 1.26+
- GNU Make
- [7-Zip](https://www.7-zip.org/) (`7z`) — used to extract the ECDICT data
- `curl` and Python 3 — used to fetch and convert the CC-CEDICT data

## Build

```sh
git clone --recursive https://github.com/major1201/translate-cli.git
cd translate-cli
make build
```

On first build, `make` prepares the dictionary data:

- extracts `dict/stardict.csv` (~232 MB) from `ECDICT/stardict.7z`
- downloads CC-CEDICT and generates `dict/cedict.csv` (~8 MB)

Both CSVs are gitignored and regenerated on demand. The resulting `tx`
binary embeds the dictionaries and is self-contained.

## Usage

```
tx [options] <text>
```

### Options

| Option | Description |
| --- | --- |
| `-s <lang>` | Source language (e.g. `en`, `zh`, `ja`). Auto-detected when omitted. |
| `-t <lang>` | Target language. Defaults to `en` for Chinese input, otherwise `zh`. |
| `-d` | Dictionary-only mode: no LLM, no API key required. |
| `-l` | LLM-only mode: skip the local dictionaries. |
| `-h` | Show help. |

`-d` and `-l` are mutually exclusive.

### Examples

```sh
$ tx apple
📖 apple  /'æpl/
   n. 苹果, 家伙

$ tx 德国
📖 德国  /dé guó/
   Germany

$ tx ドイツ                # auto-detected Japanese → Chinese
🌐 LLM Translation (ja → zh):
德国

$ tx -d good              # offline dictionary only
📖 good  /gud/
   n. 善行, 好处, 利益
   a. 好的, 优良的, 上等的, 愉快的, 有益的, 好心的, 慈善的, 虔诚的
   复数：goods
   比较级：better
   最高级：best

$ tx -l 德国              # skip the dictionary, always use the LLM
🌐 LLM Translation (zh → en):
Germany

$ tx -t ja apple          # explicit target language
🌐 LLM Translation (en → ja):
りんご
```

## LLM configuration

LLM translation requires an API key (not needed for `-d`). Configuration is
read from environment variables; the `TRAN_LLM_*` names take precedence over
the `OPENAI_*` names:

| Variable | Fallback | Default |
| --- | --- | --- |
| `TRAN_LLM_API_KEY` | `OPENAI_API_KEY` | — |
| `TRAN_LLM_BASE_URL` | `OPENAI_BASE_URL` | `https://api.openai.com/v1` |
| `TRAN_LLM_MODEL` | `OPENAI_MODEL` | `gpt-4o-mini` |

`TRAN_LLM_BASE_URL` may point to any OpenAI-compatible Chat Completions
endpoint that supports streaming:

```sh
export TRAN_LLM_API_KEY=sk-...
export TRAN_LLM_BASE_URL=https://your-provider.example.com/v1
export TRAN_LLM_MODEL=your-model
```

## How it works

1. Detect the source language from the text's Unicode script: kana → `ja`,
   Hangul → `ko`, Cyrillic → `ru`, Arabic → `ar`, Thai → `th`, Han → `zh`,
   Latin/other → `en`.
2. If the direction is covered by a local dictionary (`en→zh` or `zh→en`)
   and `-l` is not set, look up the whole text; for multi-word phrases, also
   try each word individually.
3. On a hit, print the dictionary entry and stop.
4. Otherwise, stream an LLM translation to stdout.

## Dictionary data

| Dictionary | Direction | Entries | Source | License |
| --- | --- | --- | --- | --- |
| [ECDICT](https://github.com/skywind3000/ECDICT) | en → zh | 3,402,564 | git submodule, extracted at build time | MIT |
| [CC-CEDICT](https://www.mdbg.net/chinese/export/cedict/) | zh → en | 121,175 | downloaded and converted at build time | CC BY-SA 4.0 |

## License

The bundled dictionary data is distributed under its own licenses (see the
table above).
