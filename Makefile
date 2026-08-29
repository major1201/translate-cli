.PHONY: build clean

CEDICT_URL := https://www.mdbg.net/chinese/export/cedict/cedict_1_0_ts_utf-8_mdbg.txt.gz

build: dict/stardict.csv dict/cedict.csv
	go build -o tx .

# Extract the English-Chinese dictionary from the ECDICT submodule if absent.
dict/stardict.csv:
	7z e -y -odict ECDICT/stardict.7z stardict.csv

# Download and convert the Chinese-English dictionary (CC-CEDICT) if absent.
dict/cedict.csv:
	curl -fL --retry 3 --retry-delay 2 -o /tmp/cedict_ts.gz $(CEDICT_URL)
	python3 dict/gen_cedict.py /tmp/cedict_ts.gz dict/cedict.csv
	rm -f /tmp/cedict_ts.gz

clean:
	rm -f tx dict/stardict.csv dict/cedict.csv
