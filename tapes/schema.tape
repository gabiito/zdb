# Schema browser — connect, then navigate through tables so the right-hand
# columns panel updates. Final frame lands on `students`.

Output output/schema.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

Type "ZDB_CONFIG=../test-data/config.example.toml zdb"
Enter
Sleep 1.5s

# Connect to school-sqlite
Enter
Sleep 1.5s

# Walk down the table list so each table's columns flash on the right
Down 3
Sleep 700ms
Down
Sleep 700ms
Down
Sleep 700ms
Down
Sleep 700ms

# Land on `students`
Down
Sleep 1.8s
