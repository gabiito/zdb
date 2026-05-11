.PHONY: all setup render publish clean

# Render every .tape into tapes/output/*.png
all: setup render

# Apply the SQLite test-data (idempotent) so tapes have something to show
setup:
	@command -v zdb >/dev/null 2>&1 || { echo "zdb not in PATH — run ./install.sh from repo root first" >&2; exit 1; }
	@command -v vhs >/dev/null 2>&1 || { echo "vhs not installed — go install github.com/charmbracelet/vhs@latest" >&2; exit 1; }
	@cd .. && ./test-data/apply.sh sqlite

render:
	@mkdir -p output
	@for tape in *.tape; do \
		echo "→ $$tape"; \
		vhs "$$tape" || exit 1; \
	done
	@for gif in output/*.gif; do \
		png="$${gif%.gif}.png"; \
		ffmpeg -y -sseof -0.1 -i "$$gif" -update 1 -frames:v 1 "$$png" >/dev/null 2>&1; \
	done
	@echo "✓ wrote GIFs and PNGs to tapes/output/"

publish:
	@test -d output || { echo "no output/ yet — run 'make render' first" >&2; exit 1; }
	@cp output/*.gif ../docs/screenshots/
	@echo "✓ copied $$(ls output/*.gif | wc -l) GIFs into docs/screenshots/"

clean:
	rm -rf output
