import importlib.util
import os
import unittest
from pathlib import Path


def load_app_module():
    path = Path(__file__).with_name("app.py")
    spec = importlib.util.spec_from_file_location("headroom_bridge_app", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class HeadroomBridgeTests(unittest.TestCase):
    def setUp(self):
        self.app = load_app_module()

    def test_reduce_preserves_source_identity_and_compresses_payload(self):
        previous_ratio = os.environ.get("HEADROOM_COMPRESSION_RATIO")
        os.environ["HEADROOM_COMPRESSION_RATIO"] = "0.20"
        self.app.headroom_module = lambda: object()
        try:
            payload = self.app.ReduceRequest(
                request={"ProjectID": "p"},
                skill_sections=[
                    self.app.Section(Path="skills/security-review/SKILL.md", Title="Security", Content="s" * 4000, Kind="rules")
                ],
                corpus_sections=[
                    self.app.Section(Path="docs/adr.md", Title="ADR", Content="c" * 4000, Kind="architecture")
                ],
                memory=self.app.MemoryRecall(
                    Conventions=["conv" * 1000],
                    Decisions=["decision" * 1000],
                    History=["history" * 1000],
                ),
                diff=self.app.StructuredDiff(Files=[self.app.FileDiff(Path="agent/app/server.go", Patch="+" * 4000, TokenCount=1000)]),
            )

            out = self.app.reduce(payload)

            self.assertEqual(out["skill_sections"][0]["Path"], "skills/security-review/SKILL.md")
            self.assertEqual(out["skill_sections"][0]["Title"], "Security")
            self.assertEqual(out["skill_sections"][0]["Kind"], "rules")
            self.assertLess(len(out["skill_sections"][0]["Content"]), 4000)
            self.assertEqual(out["corpus_sections"][0]["Path"], "docs/adr.md")
            self.assertLess(len(out["memory"]["Conventions"][0]), len("conv" * 1000))
            self.assertEqual(out["diff"]["Files"][0]["Path"], "agent/app/server.go")
            self.assertLess(len(out["diff"]["Files"][0]["Patch"]), 4000)
        finally:
            if previous_ratio is None:
                os.environ.pop("HEADROOM_COMPRESSION_RATIO", None)
            else:
                os.environ["HEADROOM_COMPRESSION_RATIO"] = previous_ratio

    def test_health_reports_ok_when_headroom_import_is_available(self):
        self.app.headroom_module = lambda: object()
        self.assertEqual(self.app.health(), {"status": "ok"})

    def test_compress_uses_headroom_module_when_available(self):
        class FakeHeadroom:
            @staticmethod
            def compress(text):
                return "compressed:" + text[:4]

        self.app.headroom_module = lambda: FakeHeadroom()
        self.assertEqual(self.app.compress_text("abcdef"), "compressed:abcd")


if __name__ == "__main__":
    unittest.main()
