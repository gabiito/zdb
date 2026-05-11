# Full saved-views flow in a single GIF:
#   1. Open the SQL editor
#   2. Type a query
#   3. Save it as a named view with Ctrl+S
#   4. Close the editor and reopen the views list with `V`
#   5. Final frame: the views modal with the new entry

Output output/saved-views.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

# Use an isolated /tmp config so the saved view doesn't pollute the repo
Hide
Type "cp ../test-data/config.example.toml /tmp/zdb-tape-views.toml"
Enter
Sleep 150ms
Show

Type "ZDB_CONFIG=/tmp/zdb-tape-views.toml zdb"
Enter
Sleep 1.5s

# Connect → open SQL editor from the schema browser
Enter
Sleep 1.2s
Ctrl+E
Sleep 1.2s

# Type a query
Type "SELECT * FROM students LIMIT 5"
Sleep 1s

# Save as view
Ctrl+S
Sleep 1s
Type "students-sample"
Sleep 600ms
Enter
Sleep 1.2s

# Back to schema browser, then open the saved views modal
Escape
Sleep 800ms
Type "V"
Sleep 1.8s
