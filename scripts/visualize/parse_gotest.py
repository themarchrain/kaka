"""Parse `go test -v` text output for memory-footprint tests and pass status.

Memory lines come from t.Logf in benchmarks/compare/memory_bench_test.go:
  keys=10000   kaka=327320B (32.7 B/key)  ulule=2322824B (232.3 B/key)   (TestMemoryFootprintPerKey)
  keys=100     mapStore=14408B (144.1 B/key)  lruStore=20856B (208.6 B/key)  diff=+64B/key   (TestMemoryFootprintLRU)
"""

import re
from dataclasses import dataclass

_PERKEY_RE = re.compile(
    r"keys=(\d+)\s+kaka=(\d+)B \(([\d.]+) B/key\)\s+ulule=(\d+)B \(([\d.]+) B/key\)"
)
_LRU_RE = re.compile(
    r"keys=(\d+)\s+mapStore=(\d+)B \(([\d.]+) B/key\)\s+lruStore=(\d+)B \(([\d.]+) B/key\)\s+diff=\+([\d.]+)B/key"
)
_PASS_RE = re.compile(r"^--- PASS: (\S+)")


@dataclass
class PerKeyRow:
    keys: int
    kaka_b: int
    kaka_per_key: float
    ulule_b: int
    ulule_per_key: float


@dataclass
class LRUMemRow:
    keys: int
    map_b: int
    map_per_key: float
    lru_b: int
    lru_per_key: float
    diff_per_key: float


def parse_perkey_lines(text: str) -> list[PerKeyRow]:
    out = []
    for line in text.splitlines():
        m = _PERKEY_RE.search(line)
        if m:
            out.append(PerKeyRow(
                keys=int(m.group(1)), kaka_b=int(m.group(2)), kaka_per_key=float(m.group(3)),
                ulule_b=int(m.group(4)), ulule_per_key=float(m.group(5)),
            ))
    return out


def parse_lru_mem_lines(text: str) -> list[LRUMemRow]:
    out = []
    for line in text.splitlines():
        m = _LRU_RE.search(line)
        if m:
            out.append(LRUMemRow(
                keys=int(m.group(1)), map_b=int(m.group(2)), map_per_key=float(m.group(3)),
                lru_b=int(m.group(4)), lru_per_key=float(m.group(5)),
                diff_per_key=float(m.group(6)),
            ))
    return out


def parse_pass_tests(text: str) -> list[str]:
    """Top-level passed tests, deduplicated (sub-tests stripped)."""
    names = []
    seen = set()
    for line in text.splitlines():
        m = _PASS_RE.match(line)
        if m:
            name = m.group(1).split("/")[0]
            if name not in seen:
                seen.add(name)
                names.append(name)
    return names
