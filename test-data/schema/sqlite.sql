PRAGMA foreign_keys = ON;

-- Supertype: every person in the school (students, teachers, staff)
CREATE TABLE persons (
  id            INTEGER PRIMARY KEY,
  type          TEXT    NOT NULL CHECK (type IN ('student','teacher','staff')),
  first_name    TEXT    NOT NULL,
  last_name     TEXT    NOT NULL,
  email         TEXT    NOT NULL UNIQUE,
  phone         TEXT,
  birth_date    TEXT    NOT NULL,
  address       TEXT,
  city          TEXT,
  postal_code   TEXT,
  active        INTEGER NOT NULL DEFAULT 1,
  created_at    TEXT    NOT NULL,
  updated_at    TEXT    NOT NULL
);

CREATE TABLE departments (
  id              INTEGER PRIMARY KEY,
  code            TEXT    NOT NULL UNIQUE,
  name            TEXT    NOT NULL,
  budget          NUMERIC(12,2),
  head_teacher_id INTEGER,
  created_at      TEXT    NOT NULL
);

-- Subtype tables (table-per-type inheritance)
CREATE TABLE students (
  person_id           INTEGER PRIMARY KEY REFERENCES persons(id) ON DELETE CASCADE,
  enrollment_year     INTEGER NOT NULL,
  current_grade_level INTEGER NOT NULL,
  gpa                 NUMERIC(3,2),
  scholarship         INTEGER NOT NULL DEFAULT 0,
  guardian_name       TEXT,
  guardian_phone      TEXT
);

CREATE TABLE teachers (
  person_id     INTEGER PRIMARY KEY REFERENCES persons(id) ON DELETE CASCADE,
  hire_date     TEXT    NOT NULL,
  department_id INTEGER REFERENCES departments(id),
  salary        NUMERIC(10,2) NOT NULL,
  tenure_status TEXT    NOT NULL CHECK (tenure_status IN ('probation','tenured','adjunct'))
);

CREATE TABLE staff (
  person_id     INTEGER PRIMARY KEY REFERENCES persons(id) ON DELETE CASCADE,
  hire_date     TEXT    NOT NULL,
  role          TEXT    NOT NULL,
  department_id INTEGER REFERENCES departments(id),
  salary        NUMERIC(10,2) NOT NULL
);

CREATE TABLE courses (
  id            INTEGER PRIMARY KEY,
  code          TEXT    NOT NULL UNIQUE,
  name          TEXT    NOT NULL,
  description   TEXT,
  credits       INTEGER NOT NULL,
  department_id INTEGER NOT NULL REFERENCES departments(id),
  max_students  INTEGER NOT NULL DEFAULT 30,
  teacher_id    INTEGER REFERENCES teachers(person_id)
);

CREATE TABLE enrollments (
  id          INTEGER PRIMARY KEY,
  student_id  INTEGER NOT NULL REFERENCES students(person_id),
  course_id   INTEGER NOT NULL REFERENCES courses(id),
  semester    TEXT    NOT NULL CHECK (semester IN ('spring','fall','summer')),
  year        INTEGER NOT NULL,
  status      TEXT    NOT NULL CHECK (status IN ('enrolled','completed','dropped','failed')),
  enrolled_at TEXT    NOT NULL,
  UNIQUE (student_id, course_id, semester, year)
);

CREATE TABLE grades (
  id              INTEGER PRIMARY KEY,
  enrollment_id   INTEGER NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
  assignment_name TEXT    NOT NULL,
  score           NUMERIC(5,2) NOT NULL,
  max_score       NUMERIC(5,2) NOT NULL,
  weight          NUMERIC(3,2) NOT NULL,
  graded_at       TEXT    NOT NULL
);

-- Wide table: 15 columns
CREATE TABLE attendance (
  id              INTEGER PRIMARY KEY,
  student_id      INTEGER NOT NULL REFERENCES students(person_id),
  course_id       INTEGER NOT NULL REFERENCES courses(id),
  session_date    TEXT    NOT NULL,
  status          TEXT    NOT NULL CHECK (status IN ('present','absent','late','excused')),
  arrival_time    TEXT,
  departure_time  TEXT,
  late_minutes    INTEGER NOT NULL DEFAULT 0,
  excused         INTEGER NOT NULL DEFAULT 0,
  excuse_reason   TEXT,
  parent_notified INTEGER NOT NULL DEFAULT 0,
  notified_at     TEXT,
  marked_by       INTEGER REFERENCES persons(id),
  notes           TEXT,
  created_at      TEXT    NOT NULL
);

CREATE INDEX idx_persons_type        ON persons(type);
CREATE INDEX idx_enrollments_student ON enrollments(student_id);
CREATE INDEX idx_enrollments_course  ON enrollments(course_id);
CREATE INDEX idx_attendance_student  ON attendance(student_id);
CREATE INDEX idx_attendance_course   ON attendance(course_id);
CREATE INDEX idx_attendance_date     ON attendance(session_date);
