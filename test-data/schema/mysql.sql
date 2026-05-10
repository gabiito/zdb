-- Supertype: every person in the school (students, teachers, staff)
CREATE TABLE persons (
  id            BIGINT       PRIMARY KEY,
  type          ENUM('student','teacher','staff') NOT NULL,
  first_name    VARCHAR(100) NOT NULL,
  last_name     VARCHAR(100) NOT NULL,
  email         VARCHAR(255) NOT NULL UNIQUE,
  phone         VARCHAR(50),
  birth_date    DATE         NOT NULL,
  address       VARCHAR(255),
  city          VARCHAR(100),
  postal_code   VARCHAR(20),
  active        BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at    DATETIME     NOT NULL,
  updated_at    DATETIME     NOT NULL
) ENGINE=InnoDB;

CREATE TABLE departments (
  id              BIGINT        PRIMARY KEY,
  code            VARCHAR(20)   NOT NULL UNIQUE,
  name            VARCHAR(100)  NOT NULL,
  budget          DECIMAL(12,2),
  head_teacher_id BIGINT,
  created_at      DATETIME      NOT NULL
) ENGINE=InnoDB;

-- Subtype tables (table-per-type inheritance)
CREATE TABLE students (
  person_id           BIGINT       PRIMARY KEY,
  enrollment_year     INT          NOT NULL,
  current_grade_level INT          NOT NULL,
  gpa                 DECIMAL(3,2),
  scholarship         BOOLEAN      NOT NULL DEFAULT FALSE,
  guardian_name       VARCHAR(200),
  guardian_phone      VARCHAR(50),
  CONSTRAINT students_person_fk FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE teachers (
  person_id     BIGINT        PRIMARY KEY,
  hire_date     DATE          NOT NULL,
  department_id BIGINT,
  salary        DECIMAL(10,2) NOT NULL,
  tenure_status ENUM('probation','tenured','adjunct') NOT NULL,
  CONSTRAINT teachers_person_fk FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE CASCADE,
  CONSTRAINT teachers_department_fk FOREIGN KEY (department_id) REFERENCES departments(id)
) ENGINE=InnoDB;

CREATE TABLE staff (
  person_id     BIGINT        PRIMARY KEY,
  hire_date     DATE          NOT NULL,
  role          VARCHAR(100)  NOT NULL,
  department_id BIGINT,
  salary        DECIMAL(10,2) NOT NULL,
  CONSTRAINT staff_person_fk FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE CASCADE,
  CONSTRAINT staff_department_fk FOREIGN KEY (department_id) REFERENCES departments(id)
) ENGINE=InnoDB;

ALTER TABLE departments
  ADD CONSTRAINT departments_head_teacher_fk
  FOREIGN KEY (head_teacher_id) REFERENCES teachers(person_id);

CREATE TABLE courses (
  id            BIGINT        PRIMARY KEY,
  code          VARCHAR(20)   NOT NULL UNIQUE,
  name          VARCHAR(150)  NOT NULL,
  description   TEXT,
  credits       INT           NOT NULL,
  department_id BIGINT        NOT NULL,
  max_students  INT           NOT NULL DEFAULT 30,
  teacher_id    BIGINT,
  CONSTRAINT courses_department_fk FOREIGN KEY (department_id) REFERENCES departments(id),
  CONSTRAINT courses_teacher_fk    FOREIGN KEY (teacher_id)    REFERENCES teachers(person_id)
) ENGINE=InnoDB;

CREATE TABLE enrollments (
  id          BIGINT      PRIMARY KEY,
  student_id  BIGINT      NOT NULL,
  course_id   BIGINT      NOT NULL,
  semester    ENUM('spring','fall','summer') NOT NULL,
  year        INT         NOT NULL,
  status      ENUM('enrolled','completed','dropped','failed') NOT NULL,
  enrolled_at DATETIME    NOT NULL,
  UNIQUE KEY uniq_enrollment (student_id, course_id, semester, year),
  CONSTRAINT enrollments_student_fk FOREIGN KEY (student_id) REFERENCES students(person_id),
  CONSTRAINT enrollments_course_fk  FOREIGN KEY (course_id)  REFERENCES courses(id)
) ENGINE=InnoDB;

CREATE TABLE grades (
  id              BIGINT        PRIMARY KEY,
  enrollment_id   BIGINT        NOT NULL,
  assignment_name VARCHAR(200)  NOT NULL,
  score           DECIMAL(5,2)  NOT NULL,
  max_score       DECIMAL(5,2)  NOT NULL,
  weight          DECIMAL(3,2)  NOT NULL,
  graded_at       DATETIME      NOT NULL,
  CONSTRAINT grades_enrollment_fk FOREIGN KEY (enrollment_id) REFERENCES enrollments(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- Wide table: 15 columns
CREATE TABLE attendance (
  id              BIGINT      PRIMARY KEY,
  student_id      BIGINT      NOT NULL,
  course_id       BIGINT      NOT NULL,
  session_date    DATE        NOT NULL,
  status          ENUM('present','absent','late','excused') NOT NULL,
  arrival_time    TIME,
  departure_time  TIME,
  late_minutes    INT         NOT NULL DEFAULT 0,
  excused         BOOLEAN     NOT NULL DEFAULT FALSE,
  excuse_reason   VARCHAR(255),
  parent_notified BOOLEAN     NOT NULL DEFAULT FALSE,
  notified_at     DATETIME,
  marked_by       BIGINT,
  notes           TEXT,
  created_at      DATETIME    NOT NULL,
  CONSTRAINT attendance_student_fk FOREIGN KEY (student_id) REFERENCES students(person_id),
  CONSTRAINT attendance_course_fk  FOREIGN KEY (course_id)  REFERENCES courses(id),
  CONSTRAINT attendance_marker_fk  FOREIGN KEY (marked_by)  REFERENCES persons(id)
) ENGINE=InnoDB;

CREATE INDEX idx_persons_type        ON persons(type);
CREATE INDEX idx_enrollments_student ON enrollments(student_id);
CREATE INDEX idx_enrollments_course  ON enrollments(course_id);
CREATE INDEX idx_attendance_student  ON attendance(student_id);
CREATE INDEX idx_attendance_course   ON attendance(course_id);
CREATE INDEX idx_attendance_date     ON attendance(session_date);
