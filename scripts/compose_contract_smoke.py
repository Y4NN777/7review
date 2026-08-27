#!/usr/bin/env python3
import json
import urllib.request


def request_json(url: str, payload: dict) -> dict:
    data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=240) as response:
        return json.load(response)


headroom = request_json(
    "http://127.0.0.1:8787/reduce",
    {
        "request": {"Provider": "smoke", "Title": "runtime contract"},
        "skill_sections": [
            {
                "Path": "agent/skills/methodology-review/SKILL.md",
                "Title": "Methodology",
                "Content": "Preserve evidence identity during context reduction.",
                "Kind": "rules",
            }
        ],
        "corpus_sections": [],
        "memory": {"Conventions": [], "Decisions": [], "History": []},
    },
)
sections = headroom.get("skill_sections", [])
if not sections or sections[0].get("Path") != "agent/skills/methodology-review/SKILL.md":
    raise RuntimeError(f"invalid Headroom response: {headroom}")

marker = "7review-compose-semantic-smoke"
request_json(
    "http://mempalace:8788/write",
    {
        "Conventions": ["Keep final publication human-approved."],
        "Decisions": [],
        "Vectors": [{"ID": "smoke-vector", "Text": marker, "Embedding": [1.0, 0.0]}],
    },
)
memory = request_json(
    "http://mempalace:8788/recall",
    {
        "request": {"Provider": "smoke", "Title": "semantic recall"},
        "query": marker,
        "query_embedding": [1.0, 0.0],
    },
)
if marker not in memory.get("History", []):
    raise RuntimeError(f"invalid MemPalace recall response: {memory}")

print("sidecar contracts: ok")
