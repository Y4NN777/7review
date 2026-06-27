import importlib
import os
from typing import Any

import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field


class Section(BaseModel):
    path: str = Field(default="", alias="Path")
    title: str = Field(default="", alias="Title")
    content: str = Field(default="", alias="Content")
    kind: str = Field(default="", alias="Kind")

    model_config = {"populate_by_name": True}


class FileDiff(BaseModel):
    path: str = Field(default="", alias="Path")
    patch: str = Field(default="", alias="Patch")
    token_count: int = Field(default=0, alias="TokenCount")

    model_config = {"populate_by_name": True}


class StructuredDiff(BaseModel):
    files: list[FileDiff] = Field(default_factory=list, alias="Files")

    model_config = {"populate_by_name": True}


class MemoryRecall(BaseModel):
    conventions: list[str] = Field(default_factory=list, alias="Conventions")
    decisions: list[str] = Field(default_factory=list, alias="Decisions")
    history: list[str] = Field(default_factory=list, alias="History")

    model_config = {"populate_by_name": True}


class ReduceRequest(BaseModel):
    request: dict[str, Any]
    skill_sections: list[Section] = Field(default_factory=list)
    corpus_sections: list[Section] = Field(default_factory=list)
    memory: MemoryRecall = Field(default_factory=MemoryRecall)
    diff: StructuredDiff | None = None


app = FastAPI(title="7review Headroom Bridge")


def headroom_module() -> Any:
    try:
        return importlib.import_module("headroom")
    except Exception as exc:
        raise RuntimeError(f"headroom import failed: {exc}") from exc


def compress_text(text: str) -> str:
    if not text:
        return text
    module = headroom_module()
    compress = getattr(module, "compress", None)
    if callable(compress):
        out = compress(text)
        return out if isinstance(out, str) else str(out)
    ratio = float(os.getenv("HEADROOM_COMPRESSION_RATIO", "0.55"))
    limit = max(512, int(len(text) * ratio))
    return text[:limit]


def compress_sections(sections: list[Section]) -> list[dict[str, Any]]:
    reduced = []
    for section in sections:
        reduced.append(
            {
                "Path": section.path,
                "Title": section.title,
                "Content": compress_text(section.content),
                "Kind": section.kind,
            }
        )
    return reduced


@app.get("/health")
def health() -> dict[str, str]:
    try:
        headroom_module()
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {"status": "ok"}


@app.post("/reduce")
def reduce(payload: ReduceRequest) -> dict[str, Any]:
    try:
        skill_sections = compress_sections(payload.skill_sections)
        corpus_sections = compress_sections(payload.corpus_sections)
        memory = payload.memory.model_dump(by_alias=True)
        memory["Conventions"] = [compress_text(item) for item in payload.memory.conventions]
        memory["Decisions"] = [compress_text(item) for item in payload.memory.decisions]
        memory["History"] = [compress_text(item) for item in payload.memory.history]
        diff = payload.diff.model_dump(by_alias=True) if payload.diff else None
        if diff:
            for item in diff.get("Files", []):
                item["Patch"] = compress_text(item.get("Patch", ""))
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc

    return {
        "skill_sections": skill_sections,
        "corpus_sections": corpus_sections,
        "memory": memory,
        "diff": diff,
        "warnings": [],
    }


if __name__ == "__main__":
    port = int(os.getenv("HEADROOM_BRIDGE_PORT", "8787"))
    uvicorn.run(app, host="0.0.0.0", port=port)
