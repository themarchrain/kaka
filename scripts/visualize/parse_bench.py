"""Parse `go test -bench` text output (as saved by bench.sh and friends).

Format (verified against Go 1.25.1, -benchmem):
  goos: windows / goarch: amd64 / pkg: <full> / cpu: <machine>
  BenchmarkName-N <iter> <ns/op> [<B/op> <allocs/op>]   (tab-separated)
  BenchmarkParent/child-N <iter> <ns/op>                 (sub-benchmarks, no -benchmem)
  PASS
  ok  <pkg> <duration>

Redis-unreachable runs silently omit Redis benchmark lines (stderr only),
so the parser tolerates missing lines rather than failing.
"""

import re
from dataclasses import dataclass

_BENCH_RE = re.compile(
    r"^Benchmark(\S+)-(\d+)\s+(\d+)\s+([\d.]+) ns/op(?:\s+(\d+) B/op\s+(\d+) allocs/op)?"
)
@dataclass
class BenchMeta:
    goos: str
    goarch: str
    pkg: str
    cpu: str


@dataclass
class BenchRow:
    name: str
    iterations: int
    ns_per_op: float
    bytes_per_op: int | None
    allocs_per_op: int | None


def parse_bench_text(text: str) -> tuple[BenchMeta, list[BenchRow]]:
    meta = BenchMeta("", "", "", "")
    rows: list[BenchRow] = []
    for line in text.splitlines():
        if line.startswith("goos:"):
            meta.goos = line.split(":", 1)[1].strip()
        elif line.startswith("goarch:"):
            meta.goarch = line.split(":", 1)[1].strip()
        elif line.startswith("pkg:"):
            meta.pkg = line.split(":", 1)[1].strip()
        elif line.startswith("cpu:"):
            meta.cpu = line.split(":", 1)[1].strip()
        else:
            m = _BENCH_RE.match(line)
            if m:
                bytes_per_op = int(m.group(5)) if m.group(5) is not None else None
                allocs_per_op = int(m.group(6)) if m.group(6) is not None else None
                rows.append(BenchRow(
                    name="Benchmark" + m.group(1),
                    iterations=int(m.group(3)),
                    ns_per_op=float(m.group(4)),
                    bytes_per_op=bytes_per_op,
                    allocs_per_op=allocs_per_op,
                ))
    return meta, rows


def median_rows(rows: list[BenchRow]) -> dict[str, BenchRow]:
    """Collapse repeated runs of the same benchmark (e.g. -count=3) to the
    median ns/op; B/op and allocs/op are stable so the first occurrence wins."""
    by_name: dict[str, list[BenchRow]] = {}
    for r in rows:
        by_name.setdefault(r.name, []).append(r)
    out: dict[str, BenchRow] = {}
    for name, group in by_name.items():
        group.sort(key=lambda r: r.ns_per_op)
        best = group[len(group) // 2]
        out[name] = BenchRow(
            name=name,
            iterations=best.iterations,
            ns_per_op=best.ns_per_op,
            bytes_per_op=best.bytes_per_op,
            allocs_per_op=best.allocs_per_op,
        )
    return out


def split_subbench(name: str) -> tuple[str, dict[str, str]]:
    """Split a sub-benchmark name like BenchmarkRedisConcurrencySweep/tb/parallel-1-16
    into (base_name, {"alg": "tb", "parallel": "1"}). Returns ({}, None parts)
    for flat names."""
    if "/" not in name:
        return name, {}
    parts = name.split("/")
    detail: dict[str, str] = {}
    if len(parts) >= 3 and parts[1] in ("tb", "sw", "lb"):
        detail["alg"] = parts[1]
        p = parts[2]
        if p.startswith("parallel-"):
            # "parallel-1-16": 1 = parallelism, trailing -16 = GOMAXPROCS suffix
            detail["parallel"] = p.split("-")[1]
    return parts[0], detail
