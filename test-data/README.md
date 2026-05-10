# test-data

Realistic seed data for **zDB** development. The same dataset gets loaded
into SQLite, PostgreSQL, and MySQL so you can compare driver behavior on
identical rows.

## What's in the dataset

A small school information system. Models supertype/subtype inheritance via
**table-per-type** (the only style portable across the three engines):

```
persons (supertype, 13 cols)
  ├── students    (person_id PK FK)
  ├── teachers    (person_id PK FK)
  └── staff       (person_id PK FK)

departments       (head_teacher_id FK → teachers)
courses           (department_id FK, teacher_id FK)
enrollments       (student_id FK, course_id FK)
grades            (enrollment_id FK)
attendance        ← wide table, 15 cols
```

Row counts (deterministic, seeded):

| Table       | Rows  |
|-------------|-------|
| persons     | 100   |
| students    | 60    |
| teachers    | 25    |
| staff       | 15    |
| departments | 4     |
| courses     | 8     |
| enrollments | 140   |
| grades      | 408   |
| attendance  | 1400  |

## Quick start

```bash
# 1) SQLite — no Docker needed
./apply.sh sqlite                    # writes /tmp/dev.db
./apply.sh sqlite ~/dev/school.db    # custom path

# 2) Postgres + MySQL via Docker
./apply.sh up                        # starts both containers
./apply.sh postgres                  # loads schema + data
./apply.sh mysql                     # loads schema + data
./apply.sh all                       # sqlite + postgres + mysql in one shot

# tear down (drops volumes)
./apply.sh down
```

## Run zdb against the dataset

```bash
ZDB_CONFIG=$(pwd)/test-data/config.example.toml \
ZDB_DEBUG=1 \
  go run ./cmd/zdb
```

The connection picker will show `school-sqlite`, `school-postgres`, and
`school-mysql`.

## Regenerating the data

The data file is generator output — to change volumes, names, or distributions
edit `data/generate.py` and re-run:

```bash
cd data
python3 generate.py
```

The seed is fixed (`SEED = 42`), so output is reproducible.

## Layout

```
test-data/
├── README.md
├── docker-compose.yml          # postgres:16, mysql:8
├── apply.sh                    # main entry point
├── config.example.toml         # 3 zdb connections wired up
├── schema/
│   ├── sqlite.sql
│   ├── postgres.sql
│   └── mysql.sql
└── data/
    ├── generate.py             # source of truth
    └── data.sql                # generated; portable across all 3 engines
```

## Notes on portability

`data/data.sql` works on all three engines because it sticks to the
intersection of their literal syntax:

- **Booleans**: `TRUE` / `FALSE` (SQLite ≥ 3.23, PG, MySQL all accept these).
- **Dates / times**: ISO 8601 strings (`'2024-09-01'`, `'08:30:00'`,
  `'2024-09-01 10:00:00'`).
- **NULLs and numbers**: identical on all three.
- **Identifiers**: lowercase, no reserved words.

Schema files live separately because DDL **cannot** be made portable cleanly
(types, defaults, FK timing, ENUM, INHERITS — every engine differs).
