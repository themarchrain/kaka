import io
import unittest

from visualize.parse_hey import parse_hey_csv

SAMPLE = """response-time,DNS+dialup,DNS,Request-write,Response-delay,Response-read,status-code,offset
0.0014,0.0011,0.0000,0.0000,0.0002,0.0001,200,497.7179
0.0012,0.0008,0.0000,0.0000,0.0002,0.0000,200,497.7182
0.0050,0.0010,0.0000,0.0000,0.0040,0.0000,429,497.7190
0.0200,0.0010,0.0000,0.0000,0.0190,0.0000,429,497.7210
"""


class TestParseHey(unittest.TestCase):
    def test_stats(self):
        s = parse_hey_csv(io.StringIO(SAMPLE))
        self.assertEqual(s.total, 4)
        self.assertEqual(s.allowed, 2)
        self.assertEqual(s.denied, 2)
        self.assertAlmostEqual(s.qps, 4 / (497.7210 - 497.7179), places=3)
        # 线性插值百分位：sorted=[0.0012,0.0014,0.0050,0.0200]
        self.assertAlmostEqual(s.p50, 0.0032, places=4)
        self.assertAlmostEqual(s.p95, 0.01775, places=4)
        self.assertAlmostEqual(s.p99, 0.01955, places=4)

    def test_single_row(self):
        s = parse_hey_csv(io.StringIO(SAMPLE.splitlines()[0] + "\n" + SAMPLE.splitlines()[1] + "\n"))
        self.assertEqual(s.total, 1)
        self.assertAlmostEqual(s.p50, 0.0014, places=4)


if __name__ == "__main__":
    unittest.main()
