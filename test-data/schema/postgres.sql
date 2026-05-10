-- Supertype: every person in the school (students, teachers, staff)
CREATE TABLE persons (
  id            BIGINT       PRIMARY KEY,
  type          TEXT         NOT NULL CHECK (type IN ('student','teacher','staff')),
  first_name    VARCHAR(100) NOT NULL,
  last_name     VARCHAR(100) NOT NULL,
  email         VARCHAR(255) NOT NULL UNIQUE,
  phone         VARCHAR(50),
  birth_date    DATE         NOT NULL,
  address       VARCHAR(255),
  city          VARCHAR(100),
  postal_code   VARCHAR(20),
  active        BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMP    NOT NULL,
  updated_at    TIMESTAMP    NOT NULL
);

CREATE TABLE departments (
  id              BIGINT        PRIMARY KEY,
  code            VARCHAR(20)   NOT NULL UNIQUE,
  name            VARCHAR(100)  NOT NULL,
  budget          NUMERIC(12,2),
  head_teacher_id BIGINT,
  created_at      TIMESTAMP     NOT NULL
);

-- Subtype tables (table-per-type inheritance)
CREATE TABLE students (
  person_id           BIGINT       PRIMARY KEY REFERENCES persons(id) ON DELETE CASCADE,
  enrollment_year     INTEGER      NOT NULL,
  current_grade_level INTEGER      NOT NULL,
  gpa                 NUMERIC(3,2),
  scholarship         BOOLEAN      NOT NULL DEFAULT FALSE,
  guardian_name       VARCHAR(200),
  guardian_phone      VARCHAR(50)
);

CREATE TABLE teachers (
  person_id     BIGINT        PRIMARY KEY REFERENCES persons(id) ON DELETE CASCADE,
  hire_date     DATE          NOT NULL,
  department_id BIGINT        REFERENCES departments(id),
  salary        NUMERIC(10,2) NOT NULL,
  tenure_status TEXT          NOT NULL CHECK (tenure_status IN ('probation','tenured','adjunct'))
);

CREATE TABLE staff (
  person_id     BIGINT        PRIMARY KEY REFERENCES persons(id) ON DELETE CASCADE,
  hire_date     DATE          NOT NULL,
  role          VARCHAR(100)  NOT NULL,
  department_id BIGINT        REFERENCES departments(id),
  salary        NUMERIC(10,2) NOT NULL
);

ALTER TABLE departments
  ADD CONSTRAINT departments_head_teacher_fk
  FOREIGN KEY (head_teacher_id) REFERENCES teachers(person_id);

CREATE TABLE courses (
  id            BIGINT        PRIMARY KEY,
  code          VARCHAR(20)   NOT NULL UNIQUE,
  name          VARCHAR(150)  NOT NULL,
  description   TEXT,
  credits       INTEGER       NOT NULL,
  department_id BIGINT        NOT NULL REFERENCES departments(id),
  max_students  INTEGER       NOT NULL DEFAULT 30,
  teacher_id    BIGINT        REFERENCES teachers(person_id)
);

CREATE TABLE enrollments (
  id          BIGINT      PRIMARY KEY,
  student_id  BIGINT      NOT NULL REFERENCES students(person_id),
  course_id   BIGINT      NOT NULL REFERENCES courses(id),
  semester    TEXT        NOT NULL CHECK (semester IN ('spring','fall','summer')),
  year        INTEGER     NOT NULL,
  status      TEXT        NOT NULL CHECK (status IN ('enrolled','completed','dropped','failed')),
  enrolled_at TIMESTAMP   NOT NULL,
  UNIQUE (student_id, course_id, semester, year)
);

CREATE TABLE grades (
  id              BIGINT        PRIMARY KEY,
  enrollment_id   BIGINT        NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
  assignment_name VARCHAR(200)  NOT NULL,
  score           NUMERIC(5,2)  NOT NULL,
  max_score       NUMERIC(5,2)  NOT NULL,
  weight          NUMERIC(3,2)  NOT NULL,
  graded_at       TIMESTAMP     NOT NULL
);

-- Wide table: 15 columns
CREATE TABLE attendance (
  id              BIGINT      PRIMARY KEY,
  student_id      BIGINT      NOT NULL REFERENCES students(person_id),
  course_id       BIGINT      NOT NULL REFERENCES courses(id),
  session_date    DATE        NOT NULL,
  status          TEXT        NOT NULL CHECK (status IN ('present','absent','late','excused')),
  arrival_time    TIME,
  departure_time  TIME,
  late_minutes    INTEGER     NOT NULL DEFAULT 0,
  excused         BOOLEAN     NOT NULL DEFAULT FALSE,
  excuse_reason   VARCHAR(255),
  parent_notified BOOLEAN     NOT NULL DEFAULT FALSE,
  notified_at     TIMESTAMP,
  marked_by       BIGINT      REFERENCES persons(id),
  notes           TEXT,
  created_at      TIMESTAMP   NOT NULL
);

CREATE INDEX idx_persons_type        ON persons(type);
CREATE INDEX idx_enrollments_student ON enrollments(student_id);
CREATE INDEX idx_enrollments_course  ON enrollments(course_id);
CREATE INDEX idx_attendance_student  ON attendance(student_id);
CREATE INDEX idx_attendance_course   ON attendance(course_id);
CREATE INDEX idx_attendance_date     ON attendance(session_date);
