import importlib.util
import json
import os
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace


def load_app_module():
    path = Path(__file__).with_name("app.py")
    spec = importlib.util.spec_from_file_location("mempalace_bridge_app", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class MemPalaceBridgeTests(unittest.TestCase):
    def setUp(self):
        self.app = load_app_module()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.previous_data_dir = os.environ.get("MEMPALACE_DATA_DIR")
        self.previous_namespace = os.environ.get("MEMPALACE_NAMESPACE")
        os.environ["MEMPALACE_DATA_DIR"] = self.tmp.name
        os.environ["MEMPALACE_NAMESPACE"] = "testns"
        self.addCleanup(self.restore_env)

    def restore_env(self):
        if self.previous_data_dir is None:
            os.environ.pop("MEMPALACE_DATA_DIR", None)
        else:
            os.environ["MEMPALACE_DATA_DIR"] = self.previous_data_dir
        if self.previous_namespace is None:
            os.environ.pop("MEMPALACE_NAMESPACE", None)
        else:
            os.environ["MEMPALACE_NAMESPACE"] = self.previous_namespace

    def test_health_initializes_workspace_once(self):
        calls = []

        def fake_run_cli(args):
            calls.append(args)
            return SimpleNamespace(returncode=0, stdout="", stderr="")

        self.app.require_mempalace = lambda: None
        self.app.run_cli = fake_run_cli

        self.assertEqual(self.app.health(), {"status": "ok"})
        self.assertEqual(self.app.health(), {"status": "ok"})
        self.assertEqual(calls, [["init", self.tmp.name, "--yes"]])
        self.assertEqual(Path(self.tmp.name, ".mempalace-ready").read_text(encoding="utf-8"), "ready\n")

    def test_recall_returns_bounded_history_from_cli(self):
        self.app.require_mempalace = lambda: None

        def fake_run_cli(args):
            if args[0] == "init":
                return SimpleNamespace(returncode=0, stdout="", stderr="")
            if args[0] == "search":
                return SimpleNamespace(returncode=0, stdout="\n".join(f"hit-{i}" for i in range(20)), stderr="")
            raise AssertionError(f"unexpected args {args}")

        self.app.run_cli = fake_run_cli
        out = self.app.recall(self.app.RecallRequest(request={"ProjectID": "p"}, query="auth"))

        self.assertEqual(out["Conventions"], [])
        self.assertEqual(out["Decisions"], [])
        self.assertEqual(out["History"], [f"hit-{i}" for i in range(12)])

    def test_write_persists_conventions_decisions_vectors_and_mines(self):
        calls = []
        self.app.require_mempalace = lambda: None

        def fake_run_cli(args):
            calls.append(args)
            return SimpleNamespace(returncode=0, stdout="", stderr="")

        self.app.run_cli = fake_run_cli
        payload = self.app.UpdateProposal(
            Conventions=["use bounded HTTP clients"],
            Decisions=["Headroom and MemPalace are required"],
            Vectors=[{"ID": "v1", "Text": "approved final review memory"}],
        )

        self.assertEqual(self.app.write(payload), {"status": "ok"})

        jsonl = Path(self.tmp.name, "testns.jsonl").read_text(encoding="utf-8").splitlines()
        records = [json.loads(line) for line in jsonl]
        self.assertEqual([record["kind"] for record in records], ["convention", "decision", "vector"])
        self.assertIn("Headroom and MemPalace", Path(self.tmp.name, "testns-memory.md").read_text(encoding="utf-8"))
        self.assertEqual(calls, [["init", self.tmp.name, "--yes"], ["mine", self.tmp.name]])


if __name__ == "__main__":
    unittest.main()
