# 0001. Record architecture decisions

- Status: Accepted
- Date: 2026-05-31

## Context

Trellis is a learning project with a phased build plan. As it grows, design
choices (signature scheme, package layout, storage engine, consensus rules)
accumulate, and the reasoning behind them is easy to lose. The project
principle "one concept per file" and the desire for clarity over cleverness both
benefit from a lightweight, durable record of *why* things are the way they are.

## Decision

We will record significant architecture decisions as Architecture Decision
Records (ADRs) in `docs/architecture/decisions/`, using the lightweight
Nygard format (Context / Decision / Consequences).

Each ADR is a numbered Markdown file (`NNNN-short-title.md`). New ADRs are
created from `template.md` via `make adr title="..."`, which assigns the next
sequential number, fills in the date, and sets the status to `Proposed`. An ADR
moves to `Accepted` once agreed, and to `Deprecated` or `Superseded` (noting the
superseding ADR) when no longer current.

## Consequences

- The rationale behind decisions stays close to the code and survives across
  sessions, which is valuable for a multi-phase learning project.
- Creating an ADR is one `make` command, lowering the friction that usually
  causes decision logs to rot.
- ADRs are immutable once accepted: revisiting a decision means writing a new
  ADR that supersedes the old one, rather than editing history.
- This is itself the first ADR, establishing the practice.
