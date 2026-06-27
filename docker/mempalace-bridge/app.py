import importlib
import json
import os
import subprocess
from pathlib import Path
from typing import Any

import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field


class RecallRequest(BaseModel):
    request: dict[str, Any]
    query: str


class UpdateProposal(BaseModel):
    conventions: list[str] = Field(default_factory=list, alias="Conventions")
    decisions: list[str] = Field(default_factory=list, alias="Decisions")
    vectors: list[dict[str, Any]] = Field(default_factory=list, alias="Vectors")

    model_config = {"populate_by_name": True}


app = FastAPI(title="7review MemPalace Bridge")


def data_dir() -> Path:
    path = Path(os.getenv("MEMPALACE_DATA_DIR", "/data"))
    path.mkdir(parents=True, exist_ok=True)
    return path


def require_mempalace() -> None:
    try:
        importlib.import_module("mempalace")
    except Exception as exc:
        raise RuntimeError(f"mempalace import failed: {exc}") from exc


def jsonl_path() -> Path:
    namespace = os.getenv("MEMPALACE_NAMESPACE", "7review")
    return data_dir() / f"{namespace}.jsonl"


def memory_text_path() -> Path:
    namespace = os.getenv("MEMPALACE_NAMESPACE", "7review")
    return data_dir() / f"{namespace}-memory.md"


def run_cli(args: list[str]) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    env.setdefault("MEMPALACE_HOME", str(data_dir()))
    return subprocess.run(
        ["mempalace", *args],
        check=False,
        capture_output=True,
        text=True,
        timeout=30,
        env=env,
    )


def init_workspace() -> None:
    marker = data_dir() / ".mempalace-ready"
    if marker.exists():
        return
    result = run_cli(["init", str(data_dir()), "--yes"])
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "mempalace init failed")
    marker.write_text("ready\n", encoding="utf-8")


def mine_workspace() -> None:
    result = run_cli(["mine", str(data_dir())])
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "mempalace mine failed")


def write_item(kind: str, text: str) -> None:
    if not text:
        return
    record = {"kind": kind, "text": text}
    with jsonl_path().open("a", encoding="utf-8") as fh:
        fh.write(json.dumps(record, ensure_ascii=True) + "\n")
    with memory_text_path().open("a", encoding="utf-8") as fh:
        fh.write(f"\n## {kind}\n\n{text}\n")


def recall_from_cli(query: str) -> list[str]:
    result = run_cli(["search", query])
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "mempalace search failed")
    lines = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    return lines[:12]


@app.get("/health")
def health() -> dict[str, str]:
    try:
        require_mempalace()
        init_workspace()
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {"status": "ok"}


@app.post("/recall")
def recall(payload: RecallRequest) -> dict[str, list[str]]:
    try:
        require_mempalace()
        init_workspace()
        history = recall_from_cli(payload.query)
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {"Conventions": [], "Decisions": [], "History": history}


@app.post("/write")
def write(payload: UpdateProposal) -> dict[str, str]:
    try:
        require_mempalace()
        init_workspace()
        for item in payload.conventions:
            write_item("convention", item)
        for item in payload.decisions:
            write_item("decision", item)
        for vector in payload.vectors:
            write_item("vector", vector.get("Text") or vector.get("text") or "")
        mine_workspace()
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {"status": "ok"}


if __name__ == "__main__":
    port = int(os.getenv("MEMPALACE_BRIDGE_PORT", "8788"))
    uvicorn.run(app, host="0.0.0.0", port=port)
