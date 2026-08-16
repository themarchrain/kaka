import unittest

from visualize.parse_bench import (
    BenchMeta, BenchRow, parse_bench_text, median_rows, split_subbench,
)

SAMPLE = """goos: windows
goarch: amd64
pkg: github.com/themarchrain/kaka/benchmarks/compare
cpu: 13th Gen Intel(R) Core(TM) i7-13620H
BenchmarkKakaTokenBucketHotPath-16    \t     200\t        48.50 ns/op\t       0 B/op\t       0 allocs/op
BenchmarkKakaTokenBucketHotPath-16    \t     200\t        27.50 ns/op\t       0 B/op\t       0 allocs/op
BenchmarkKakaTokenBucketHotPath-16    \t     200\t        26.00 ns/op\t       0 B/op\t       0 allocs/op
BenchmarkRedisConcurrencySweep/tb/parallel-1-16          \t       1\t   1533600 ns/op
BenchmarkRedisConcurrencySweep/sw/parallel-64-16         \t       1\t     27400 ns/op
BenchmarkUluleRedisParallel64-16      \t     100\t   15800.0 ns/op\t     406 B/op\t      10 allocs/op
PASS
ok  \tgithub.com/themarchrain/kaka/benchmarks/compare\t5.571s
"""


class TestParseBench(unittest.TestCase):
    def test_meta(self):
        meta, rows = parse_bench_text(SAMPLE)
        self.assertEqual(meta.goos, "windows")
        self.assertEqual(meta.goarch, "amd64")
        self.assertIn("i7-13620H", meta.cpu)

    def test_row_count_and_fields(self):
        meta, rows = parse_bench_text(SAMPLE)
        self.assertEqual(len(rows), 6)
        r = rows[0]
        self.assertEqual(r.name, "BenchmarkKakaTokenBucketHotPath")
        self.assertEqual(r.ns_per_op, 48.5)
        self.assertEqual(r.bytes_per_op, 0)
        self.assertEqual(r.allocs_per_op, 0)

    def test_no_benchmem_columns(self):
        meta, rows = parse_bench_text(SAMPLE)
        sweep = [r for r in rows if "Sweep" in r.name]
        self.assertEqual(len(sweep), 2)
        self.assertIsNone(sweep[0].bytes_per_op)

    def test_median_rows(self):
        meta, rows = parse_bench_text(SAMPLE)
        med = median_rows(rows)
        self.assertEqual(med["BenchmarkKakaTokenBucketHotPath"].ns_per_op, 27.5)
        self.assertEqual(len(med), 4)  # 3 同名合并为 1（样本共 4 个唯一名）

    def test_split_subbench(self):
        base, parts = split_subbench("BenchmarkRedisConcurrencySweep/tb/parallel-1-16")
        self.assertEqual(base, "BenchmarkRedisConcurrencySweep")
        self.assertEqual(parts, {"alg": "tb", "parallel": "1"})

    def test_tolerates_missing_lines(self):
        meta, rows = parse_bench_text("goos: linux\ngoarch: amd64\npkg: x\ncpu: y\nPASS\nok  \tx\t1s\n")
        self.assertEqual(len(rows), 0)


if __name__ == "__main__":
    unittest.main()
