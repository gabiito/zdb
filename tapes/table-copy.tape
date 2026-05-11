# Data viewer with a range of rows marked.
# Opens a table, anchors a mark with Space, moves down, then uses `M`
# to extend the range — final frame shows multiple highlighted rows
# and the `Y copy N marked` hint in the help bar.

Output output/table-copy.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

Type "ZDB_CONFIG=../test-data/config.example.toml zdb"
Enter
Sleep 1.5s

# Connect → open first table (attendance)
Enter
Sleep 1.2s
Enter
Sleep 1.8s

# Anchor a mark
Down 2
Sleep 400ms
Space
Sleep 700ms

# Move further down and extend the range
Down 4
Sleep 400ms
Type "M"
Sleep 1.8s
