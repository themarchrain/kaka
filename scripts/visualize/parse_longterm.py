"""Parse longterm memory sampling files written by scripts/loadtest-long.sh.

The header line is CSV-shaped (ts,heapAlloc,...) but every data row is a
space-separated key=value line, e.g.:
  ts=0 goroutines=3 heapAlloc=360760 heapObjects=1390 numGC=0
Parse rows by the key=value format; the header is ignored.
"""

import pandas as pd

_KEYS = ("ts", "heapAlloc", "heapObjects", "numGC", "goroutines")


def parse_longterm_mem(text: str) -> pd.DataFrame:
    records = []
    for line in text.splitlines():
        if "=" not in line or line.startswith("ts,"):
            continue
        kv = {}
        for token in line.split():
            if "=" in token:
                k, _, v = token.partition("=")
                kv[k] = int(v)
        if "ts" in kv:
            records.append(kv)
    return pd.DataFrame(records, columns=list(_KEYS))
