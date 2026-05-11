# Full staged-edit flow in a single GIF:
#   1. Open a table
#   2. Edit a cell, save the change → it lands in the staged buffer
#   3. Press `S` to open the review modal
#   4. Final frame: the review modal listing the pending edit

Output output/staged-edit.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

Type "ZDB_CONFIG=../test-data/config.example.toml zdb"
Enter
Sleep 1.5s

# Connect → open attendance
Enter
Sleep 1.2s
Enter
Sleep 1.8s

# Navigate to the session_date column (col 3) on row 3
Down 2
Sleep 300ms
Right 3
Sleep 600ms

# Open the edit modal
Enter
Sleep 1s

# Clear the existing date (10 chars: "2024-08-29") and type a new one
Backspace 10
Sleep 300ms
Type "2025-01-01"
Sleep 1s

# Stage the edit
Enter
Sleep 1.2s

# Open the staged-edits review modal
Type "S"
Sleep 2s
