# tapes — auto-generated screenshots

This directory holds [`vhs`](https://github.com/charmbracelet/vhs) tape files
that script a headless zDB session and produce both an animated **GIF** of the
interaction and a static **PNG** of the final frame (extracted with `ffmpeg`).
Used to keep the docs in sync with the UI without retaking screenshots by hand.

> vhs v0.11 only emits gif/mp4/webm natively, so the Makefile post-processes
> each GIF into a PNG via `ffmpeg -sseof -0.1 -update 1 -frames:v 1`.

> **Separate from the app.** Nothing in `tapes/` is compiled into the binary or
> shipped to users. It's a dev workflow for regenerating documentation assets.

## What is and isn't tracked

- **Committed**: the `.tape` source files, this README, and the Makefile.
- **Gitignored**: `output/` (the generated PNGs) — regenerable from the tapes.

If you want the PNGs in git too (so consumers don't need vhs installed), drop
the `output/` line from `tapes/.gitignore`.

## Prerequisites

```sh
go install github.com/charmbracelet/vhs@latest    # vhs binary
./install.sh                                       # zdb binary, from repo root
```

vhs also needs `ffmpeg` and `ttyd` on your `$PATH` — see
[vhs install docs](https://github.com/charmbracelet/vhs#installation) for
platform notes.

## Render everything

```sh
cd tapes
make            # applies test-data (SQLite, no Docker) and runs every tape
```

Output lands in `tapes/output/*.png`.

## Render one tape

```sh
cd tapes
vhs schema.tape       # writes output/schema.png
```

## What each tape captures

| Tape | View |
|---|---|
| `schema.tape` | Schema browser — walks down the table list so the columns panel updates |
| `table-copy.tape` | Data viewer with a 5-row range selected via `Space` + `M` |
| `staged-edit.tape` | Edit a cell, save → review modal with the pending change |
| `saved-views.tape` | SQL editor → save query as view → open views list |
| `ai-flow.tape` | AI profiles list → analytics dashboard (profile pre-baked in setup) |

## What can't be auto-generated

These remain manual screenshots in `docs/screenshots/`:

- **Ask AI** input + result — needs a live API key and a network round-trip;
  responses are non-deterministic.
- **AI analytics** dashboard — needs prior request history in
  `~/.local/state/zdb/ai-usage.jsonl`.

If you want to script those too, set up a profile pointing at a local Ollama
endpoint with a fixed seed and add an `ai-ask.tape` — but that's optional.

## Updating the docs

The docs reference `docs/screenshots/*.gif` (animated demos) plus a couple
of static PNGs that can't be auto-generated. To refresh the docs from the
latest tape outputs:

```sh
cd tapes
make            # render every tape into output/
make publish    # copy the GIFs into ../docs/screenshots/
```

Then commit the updated `docs/screenshots/*.gif` files.
