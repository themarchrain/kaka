import tempfile
import unittest
from pathlib import Path

from visualize.main import CHARTS, KINDS, newest


def _make_kind_file(raw_dir: Path, pattern: str, name: str) -> Path:
    """Create a file that a kind's glob must match, and assert siblings don't."""
    p = raw_dir / name
    p.write_text("x")
    return p


class TestKindsGlobs(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.raw = Path(self.tmp.name)

    def tearDown(self):
        self.tmp.cleanup()

    def test_each_kind_glob_matches_its_own_filename(self):
        samples = {
            "bench": "benchmark-20260815-231056.txt",
            "loadtest-kaka": "loadtest-kaka-20260815-231030.csv",
            "loadtest-xtime": "loadtest-xtime-20260815-231030.csv",
            "loadtest-kaka-many": "loadtest-kaka-many-20260815-231030.csv",
            "loadtest-redis": "loadtest-redis-20260815-231908.csv",
            "loadtest-layered": "loadtest-layered-20260815-231908.csv",
            "memory-perkey": "memory-perkey-20260815-230949.txt",
            "memory-lru": "memory-lru-20260815-230949.txt",
            "longterm-mem": "longterm-mem-20260815-231030.csv",
            "correctness": "correctness-20260815-230949.txt",
            "lru": "lru-20260815-230956.txt",
            "bench-redis": "benchmark-redis-20260815-231522.txt",
            "bench-redis-sweep": "benchmark-redis-sweep-20260815-231522.txt",
            "bench-layered": "benchmark-layered-20260815-231908.txt",
        }
        for kind, filename in samples.items():
            _make_kind_file(self.raw, KINDS[kind], filename)
            with self.subTest(kind=kind):
                self.assertIsNotNone(newest(self.raw, KINDS[kind]),
                                     f"{kind} glob {KINDS[kind]} must match {filename}")

    def test_glob_rejects_sibling_kinds(self):
        """The exact regression from commit 75535f6: unanchored globs capture
        sibling kinds (benchmark-redis-* satisfying bench, etc.)."""
        # Sibling files that must NOT satisfy these kinds:
        self.assertIsNone(newest(self.raw, KINDS["bench"]))  # empty dir first
        (self.raw / "benchmark-redis-sweep-20260815-231522.txt").write_text("x")
        (self.raw / "benchmark-redis-20260815-231522.txt").write_text("x")
        (self.raw / "benchmark-layered-20260815-231908.txt").write_text("x")
        self.assertIsNone(newest(self.raw, KINDS["bench"]),
                          "bench must not match redis/layered siblings")
        (self.raw / "loadtest-kaka-many-20260815-231030.csv").write_text("x")
        self.assertIsNone(newest(self.raw, KINDS["loadtest-kaka"]),
                          "loadtest-kaka must not match loadtest-kaka-many")

    def test_newest_returns_latest(self):
        import os
        import time
        a = self.raw / "benchmark-20260815-100000.txt"
        b = self.raw / "benchmark-20260815-200000.txt"
        a.write_text("a")
        b.write_text("b")
        # Same-second writes share mtime on some filesystems; pin them explicitly.
        now = time.time()
        os.utime(a, (now, now))
        os.utime(b, (now + 60, now + 60))
        self.assertEqual(newest(self.raw, KINDS["bench"]).name, b.name)

    def test_charts_mapping_is_complete(self):
        self.assertEqual(len(CHARTS), 13)


if __name__ == "__main__":
    unittest.main()
