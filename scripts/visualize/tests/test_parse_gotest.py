import unittest

from visualize.parse_gotest import parse_perkey_lines, parse_lru_mem_lines, parse_pass_tests

PERKEY = """=== RUN   TestMemoryFootprintPerKey
    memory_bench_test.go:55: keys=100     kaka=321288B (3212.9 B/key)  ulule=346312B (3463.1 B/key)
    memory_bench_test.go:55: keys=10000   kaka=327320B (32.7 B/key)  ulule=2322824B (232.3 B/key)
--- PASS: TestMemoryFootprintPerKey (0.08s)
PASS
"""

LRU = """=== RUN   TestMemoryFootprintLRU
    memory_bench_test.go:96: keys=100     mapStore=14408B (144.1 B/key)  lruStore=20856B (208.6 B/key)  diff=+64B/key
    memory_bench_test.go:96: keys=100000  mapStore=15837864B (158.4 B/key)  lruStore=22191688B (221.9 B/key)  diff=+63B/key
--- PASS: TestMemoryFootprintLRU (0.07s)
PASS
"""

CORRECTNESS = """=== RUN   TestTokenBucket_DifferentialCorrectness
=== RUN   TestTokenBucket_DifferentialCorrectness/steady_rate
--- PASS: TestTokenBucket_DifferentialCorrectness (0.02s)
--- PASS: TestTokenBucket_DifferentialCorrectness/steady_rate (0.00s)
--- PASS: TestTokenBucket_PerKeyIsolationConsistency (0.01s)
PASS
"""


class TestParseGotest(unittest.TestCase):
    def test_perkey(self):
        rows = parse_perkey_lines(PERKEY)
        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0].keys, 100)
        self.assertAlmostEqual(rows[0].kaka_per_key, 3212.9, places=1)
        self.assertAlmostEqual(rows[1].ulule_per_key, 232.3, places=1)

    def test_lru_mem(self):
        rows = parse_lru_mem_lines(LRU)
        self.assertEqual(len(rows), 2)
        self.assertAlmostEqual(rows[0].diff_per_key, 64.0, places=1)
        self.assertAlmostEqual(rows[1].lru_per_key, 221.9, places=1)

    def test_pass_tests_dedupe_parent(self):
        names = parse_pass_tests(CORRECTNESS)
        self.assertIn("TestTokenBucket_DifferentialCorrectness", names)
        self.assertIn("TestTokenBucket_PerKeyIsolationConsistency", names)
        self.assertEqual(names.count("TestTokenBucket_DifferentialCorrectness"), 1)

    def test_empty(self):
        self.assertEqual(parse_perkey_lines("nothing here\n"), [])
        self.assertEqual(parse_pass_tests("nothing here\n"), [])


if __name__ == "__main__":
    unittest.main()
