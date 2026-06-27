#!/usr/bin/env python3
import json
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: validate_context_pack.py <context_pack.json>", file=sys.stderr)
        return 2
    path = Path(sys.argv[1])
    if not path.exists():
        print(f"{path} does not exist", file=sys.stderr)
        return 1
    data = json.loads(path.read_text(encoding="utf-8"))
    missing = [key for key in ("project", "generated_at", "sections") if key not in data]
    if missing:
        print(f"missing top-level keys: {', '.join(missing)}", file=sys.stderr)
        return 1
    if not isinstance(data["sections"], list):
        print("sections must be a list", file=sys.stderr)
        return 1
    for idx, section in enumerate(data["sections"]):
        for key in ("id", "title", "kind", "paths", "content", "review_triggers"):
            if key not in section:
                print(f"section {idx} missing {key}", file=sys.stderr)
                return 1
    print("context pack ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
