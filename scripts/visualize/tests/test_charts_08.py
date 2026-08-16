import unittest

from visualize.charts_08 import parse_multikey_subbench


class TestParseMultikeySubbench(unittest.TestCase):
    # parse_bench strips the -<GOMAXPROCS> suffix, so names carry no trailing -16.
    def test_parses_g_and_k(self):
        self.assertEqual(parse_multikey_subbench("BenchmarkMultiKeyParallel/g8k64"),
                         ("BenchmarkMultiKeyParallel", 8, 64))

    def test_parses_single_worker(self):
        self.assertEqual(parse_multikey_subbench("BenchmarkMultiKeyParallel/g1k8"),
                         ("BenchmarkMultiKeyParallel", 1, 8))

    def test_rejects_unrelated_benchmarks(self):
        self.assertIsNone(parse_multikey_subbench("BenchmarkRedisTokenBucket-16"))
        self.assertIsNone(parse_multikey_subbench("BenchmarkMultiKeyParallel-16"))

    def test_rejects_malformed_subbench(self):
        self.assertIsNone(parse_multikey_subbench("BenchmarkMultiKeyParallel/g8"))
        self.assertIsNone(parse_multikey_subbench("BenchmarkMultiKeyParallel/g8x64"))


if __name__ == "__main__":
    unittest.main()
