# AI profile listing → analytics, in a single GIF.
#
# Pre-populates an AI profile in an isolated /tmp config before launching
# zdb. The wizard's interactive save flow is tricky in vhs's headless
# ttyd (post-save Ask Panel hijacks focus, keyring calls fail silently),
# so we bake the profile directly into the TOML. The narrative captured:
#
#   1. zdb starts with an `openai` profile already configured
#   2. Ctrl+P opens the AI profiles list — the profile is visible
#   3. `g` jumps to the analytics dashboard

Output output/ai-flow.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

Hide
Type "cp ../test-data/config.example.toml /tmp/zdb-tape-ai.toml"
Enter
Sleep 150ms
Type 'printf "\nactive_ai = \"openai\"\n\n[[ais]]\nname = \"openai\"\nprovider = \"openai-compat\"\nbase_url = \"https://api.openai.com/v1\"\nmodel = \"gpt-4o-mini\"\nkeyring_key = \"zdb/ai-key/openai\"\n" >> /tmp/zdb-tape-ai.toml'
Enter
Sleep 150ms
Show

Type "ZDB_CONFIG=/tmp/zdb-tape-ai.toml zdb"
Enter
Sleep 1.5s

# Connect to school-sqlite
Enter
Sleep 1.2s

# Open the AI profiles list — the pre-populated profile is visible
Ctrl+P
Sleep 2s

# Jump to the analytics dashboard
Type "g"
Sleep 2.5s
