import unittest

from visualize.parse_longterm import parse_longterm_mem

SAMPLE = """ts,heapAlloc,heapObjects,numGC,goroutines
ts=0 goroutines=3 heapAlloc=360760 heapObjects=1390 numGC=0
ts=10 goroutines=4 heapAlloc=2200000 heapObjects=50000 numGC=3
ts=20 goroutines=4 heapAlloc=3100000 heapObjects=52000 numGC=5
"""


class TestParseLongterm(unittest.TestCase):
    def test_parses_keyvalue_rows(self):
        df = parse_longterm_mem(SAMPLE)
        self.assertEqual(len(df), 3)
        self.assertEqual(list(df.columns), ["ts", "heapAlloc", "heapObjects", "numGC", "goroutines"])
        self.assertEqual(df.iloc[0]["ts"], 0)
        self.assertEqual(df.iloc[1]["heapAlloc"], 2200000)
        self.assertEqual(df.iloc[2]["numGC"], 5)

    def test_empty(self):
        df = parse_longterm_mem("")
        self.assertEqual(len(df), 0)


if __name__ == "__main__":
    unittest.main()
