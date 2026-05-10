#!/usr/bin/env python3
"""Generate portable seed data for the zDB school schema.

Output: data.sql in this directory. The SQL is written to be portable across
SQLite, PostgreSQL, and MySQL (uses TRUE/FALSE keywords, ISO 8601 dates and
times, single-quoted strings, explicit IDs).

Usage: python3 generate.py
"""

from __future__ import annotations

import datetime as dt
import random
from pathlib import Path

SEED = 42
random.seed(SEED)

OUT = Path(__file__).parent / "data.sql"

FIRST_NAMES = [
    "Ada", "Alan", "Grace", "Linus", "Donald", "Edsger", "Tony", "Margaret",
    "Hedy", "Barbara", "Niklaus", "Bjarne", "Ken", "Dennis", "James", "Brian",
    "Rob", "Anders", "Guido", "Yukihiro", "John", "Joan", "Carmela", "Sofia",
    "Mateo", "Lucia", "Valentina", "Tomas", "Camila", "Joaquin", "Renata",
    "Bruno", "Olivia", "Liam", "Noah", "Emma", "Ava", "Isabella", "Mia",
    "Ethan", "Mason", "Logan", "Lucas", "Aiden", "Jackson", "Amelia",
    "Harper", "Evelyn", "Abigail", "Nora", "Mila", "Eleanor", "Hazel",
    "Lily", "Aria", "Layla", "Zoe", "Penelope", "Riley", "Stella",
    "Aurora", "Violet", "Skylar", "Lucy", "Anna", "Caroline", "Genesis",
    "Hannah", "Ariana", "Allison", "Gabriella", "Alice", "Madelyn", "Cora",
    "Ruby", "Eva", "Serenity", "Autumn", "Adeline", "Hailey", "Gianna",
    "Valeria", "Isla", "Eliana", "Quinn", "Nevaeh", "Ivy", "Sadie",
    "Piper", "Lydia", "Alexa", "Josephine", "Emilia", "Athena", "Vivian",
    "Delilah", "Melanie", "Iris", "Daniela", "Catalina",
]

LAST_NAMES = [
    "Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller",
    "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez",
    "Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin",
    "Lee", "Perez", "Thompson", "White", "Harris", "Sanchez", "Clark",
    "Ramirez", "Lewis", "Robinson", "Walker", "Young", "Allen", "King",
    "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores", "Green",
    "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell",
    "Carter", "Roberts", "Gomez", "Phillips", "Evans", "Turner", "Diaz",
    "Parker", "Cruz", "Edwards", "Collins", "Reyes", "Stewart", "Morris",
    "Morales", "Murphy", "Cook", "Rogers", "Gutierrez", "Ortiz", "Morgan",
    "Cooper", "Peterson", "Bailey", "Reed", "Kelly", "Howard", "Ramos",
    "Kim", "Cox", "Ward", "Richardson", "Watson", "Brooks", "Chavez",
]

CITIES = [
    "Buenos Aires", "Cordoba", "Rosario", "Mendoza", "La Plata", "Mar del Plata",
    "Tucuman", "Salta", "Santa Fe", "Bahia Blanca",
]

STREETS = [
    "Av. Corrientes", "Av. Cabildo", "Av. Santa Fe", "Av. de Mayo", "Av. Rivadavia",
    "Calle Florida", "Av. 9 de Julio", "Av. Las Heras", "Calle Lavalle", "Av. Belgrano",
]

DEPARTMENTS = [
    ("SCI", "Sciences",   850000.00),
    ("MAT", "Mathematics", 620000.00),
    ("HUM", "Humanities",  540000.00),
    ("ART", "Arts",        310000.00),
]

COURSES = [
    # (code, name, description, credits, department_idx, max_students)
    ("BIO101", "Introduction to Biology",
     "Foundations of cellular and molecular biology.", 4, 0, 30),
    ("PHY201", "Classical Mechanics",
     "Newtonian mechanics, energy, and momentum.",     5, 0, 25),
    ("MAT101", "Calculus I",
     "Limits, derivatives, and basic integrals.",      4, 1, 30),
    ("MAT301", "Linear Algebra",
     "Vectors, matrices, and linear transformations.", 4, 1, 25),
    ("HIS210", "World History",
     "From antiquity to the modern era.",              3, 2, 35),
    ("LIT220", "Latin American Literature",
     "Survey of major works and authors.",             3, 2, 30),
    ("MUS110", "Music Theory",
     "Notation, harmony, and ear training.",           2, 3, 20),
    ("ART150", "Drawing Fundamentals",
     "Line, shape, perspective, and shading.",         2, 3, 20),
]

ASSIGNMENTS = [
    ("Midterm Exam",   100, 0.30),
    ("Final Exam",     100, 0.40),
    ("Term Project",    50, 0.30),
]

ATTENDANCE_STATUS_WEIGHTS = [
    ("present",  70),
    ("late",     12),
    ("absent",   10),
    ("excused",   8),
]

EXCUSE_REASONS = [
    "doctor appointment", "family emergency", "school sports event",
    "field trip permission", "religious observance",
]


def sql_str(s: str | None) -> str:
    if s is None:
        return "NULL"
    return "'" + s.replace("'", "''") + "'"


def sql_bool(b: bool) -> str:
    return "TRUE" if b else "FALSE"


def sql_num(n) -> str:
    if n is None:
        return "NULL"
    return str(n)


def random_date(start: dt.date, end: dt.date) -> dt.date:
    delta = (end - start).days
    return start + dt.timedelta(days=random.randint(0, delta))


def weighted_choice(pairs):
    total = sum(w for _, w in pairs)
    r = random.uniform(0, total)
    upto = 0
    for value, w in pairs:
        upto += w
        if upto >= r:
            return value
    return pairs[-1][0]


def make_email(first: str, last: str, idx: int, domain: str) -> str:
    return f"{first.lower()}.{last.lower()}{idx}@{domain}"


# ── Generation ────────────────────────────────────────────────────────────────

NOW = dt.datetime(2024, 9, 1, 9, 0, 0)
ACADEMIC_START = dt.date(2024, 8, 15)

persons = []   # list of dicts with id, type, ...
students = []  # list of person_ids
teachers = []
staff = []

# 100 persons: 60 students, 25 teachers, 15 staff
def gen_persons():
    pid = 1
    used_emails = set()

    for _ in range(60):
        first = random.choice(FIRST_NAMES)
        last  = random.choice(LAST_NAMES)
        domain = "students.school.edu"
        email = make_email(first, last, pid, domain)
        while email in used_emails:
            pid += 0  # noop, just regenerate
            email = make_email(first, last, pid + random.randint(1000, 9999), domain)
        used_emails.add(email)
        birth = random_date(dt.date(2006, 1, 1), dt.date(2010, 12, 31))
        persons.append({
            "id": pid,
            "type": "student",
            "first_name": first,
            "last_name": last,
            "email": email,
            "phone": f"+54 11 4{random.randint(100,999)}-{random.randint(1000,9999)}",
            "birth_date": birth.isoformat(),
            "address": f"{random.choice(STREETS)} {random.randint(100,9999)}",
            "city": random.choice(CITIES),
            "postal_code": f"C{random.randint(1000,1999)}ABC",
            "active": random.random() > 0.05,
            "created_at": (NOW - dt.timedelta(days=random.randint(30, 800))).isoformat(sep=" "),
            "updated_at": (NOW - dt.timedelta(days=random.randint(0, 30))).isoformat(sep=" "),
        })
        students.append(pid)
        pid += 1

    for _ in range(25):
        first = random.choice(FIRST_NAMES)
        last  = random.choice(LAST_NAMES)
        email = make_email(first, last, pid, "faculty.school.edu")
        used_emails.add(email)
        birth = random_date(dt.date(1965, 1, 1), dt.date(1995, 12, 31))
        persons.append({
            "id": pid,
            "type": "teacher",
            "first_name": first,
            "last_name": last,
            "email": email,
            "phone": f"+54 11 5{random.randint(100,999)}-{random.randint(1000,9999)}",
            "birth_date": birth.isoformat(),
            "address": f"{random.choice(STREETS)} {random.randint(100,9999)}",
            "city": random.choice(CITIES),
            "postal_code": f"C{random.randint(1000,1999)}DEF",
            "active": random.random() > 0.02,
            "created_at": (NOW - dt.timedelta(days=random.randint(365, 4000))).isoformat(sep=" "),
            "updated_at": (NOW - dt.timedelta(days=random.randint(0, 90))).isoformat(sep=" "),
        })
        teachers.append(pid)
        pid += 1

    for _ in range(15):
        first = random.choice(FIRST_NAMES)
        last  = random.choice(LAST_NAMES)
        email = make_email(first, last, pid, "staff.school.edu")
        used_emails.add(email)
        birth = random_date(dt.date(1970, 1, 1), dt.date(2000, 12, 31))
        persons.append({
            "id": pid,
            "type": "staff",
            "first_name": first,
            "last_name": last,
            "email": email,
            "phone": f"+54 11 6{random.randint(100,999)}-{random.randint(1000,9999)}",
            "birth_date": birth.isoformat(),
            "address": f"{random.choice(STREETS)} {random.randint(100,9999)}",
            "city": random.choice(CITIES),
            "postal_code": f"C{random.randint(1000,1999)}GHI",
            "active": random.random() > 0.05,
            "created_at": (NOW - dt.timedelta(days=random.randint(180, 3000))).isoformat(sep=" "),
            "updated_at": (NOW - dt.timedelta(days=random.randint(0, 60))).isoformat(sep=" "),
        })
        staff.append(pid)
        pid += 1


def gen_departments():
    out = []
    for i, (code, name, budget) in enumerate(DEPARTMENTS, start=1):
        out.append({
            "id": i,
            "code": code,
            "name": name,
            "budget": budget,
            "head_teacher_id": None,  # set after teachers exist
            "created_at": "2020-01-15 09:00:00",
        })
    return out


def gen_student_subtype():
    out = []
    for sid in students:
        out.append({
            "person_id": sid,
            "enrollment_year": random.choice([2022, 2023, 2024]),
            "current_grade_level": random.randint(7, 12),
            "gpa": round(random.uniform(2.00, 4.00), 2),
            "scholarship": random.random() < 0.20,
            "guardian_name": f"{random.choice(FIRST_NAMES)} {random.choice(LAST_NAMES)}",
            "guardian_phone": f"+54 11 7{random.randint(100,999)}-{random.randint(1000,9999)}",
        })
    return out


def gen_teacher_subtype():
    out = []
    for tid in teachers:
        dept_id = random.randint(1, len(DEPARTMENTS))
        hire = random_date(dt.date(2005, 1, 1), dt.date(2023, 12, 31))
        out.append({
            "person_id": tid,
            "hire_date": hire.isoformat(),
            "department_id": dept_id,
            "salary": round(random.uniform(45000, 95000), 2),
            "tenure_status": random.choice(["probation", "tenured", "tenured", "adjunct"]),
        })
    return out


def gen_staff_subtype():
    out = []
    roles = ["registrar", "librarian", "counselor", "janitor", "nurse",
             "it support", "security", "groundskeeper"]
    for sid in staff:
        out.append({
            "person_id": sid,
            "hire_date": random_date(dt.date(2010, 1, 1), dt.date(2024, 6, 1)).isoformat(),
            "role": random.choice(roles),
            "department_id": random.choice([None, 1, 2, 3, 4]),
            "salary": round(random.uniform(28000, 55000), 2),
        })
    return out


def gen_courses(teacher_subtype):
    out = []
    teachers_by_dept: dict[int, list[int]] = {}
    for t in teacher_subtype:
        teachers_by_dept.setdefault(t["department_id"], []).append(t["person_id"])
    for i, (code, name, desc, credits, dept_idx, mx) in enumerate(COURSES, start=1):
        dept_id = dept_idx + 1
        pool = teachers_by_dept.get(dept_id) or [t["person_id"] for t in teacher_subtype]
        out.append({
            "id": i,
            "code": code,
            "name": name,
            "description": desc,
            "credits": credits,
            "department_id": dept_id,
            "max_students": mx,
            "teacher_id": random.choice(pool),
        })
    return out


def gen_enrollments(courses):
    out = []
    eid = 1
    for sid in students:
        chosen = random.sample([c["id"] for c in courses], k=random.choice([2, 2, 3]))
        for cid in chosen:
            out.append({
                "id": eid,
                "student_id": sid,
                "course_id": cid,
                "semester": "fall",
                "year": 2024,
                "status": weighted_choice([
                    ("enrolled", 70), ("completed", 20),
                    ("dropped", 5), ("failed", 5),
                ]),
                "enrolled_at": (NOW - dt.timedelta(days=random.randint(15, 60)))
                                .isoformat(sep=" "),
            })
            eid += 1
    return out


def gen_grades(enrollments):
    out = []
    gid = 1
    for e in enrollments:
        if e["status"] == "dropped":
            continue
        for assignment, max_score, weight in ASSIGNMENTS:
            score = round(random.uniform(40, max_score), 2)
            out.append({
                "id": gid,
                "enrollment_id": e["id"],
                "assignment_name": assignment,
                "score": score,
                "max_score": max_score,
                "weight": weight,
                "graded_at": (NOW - dt.timedelta(days=random.randint(0, 90)))
                              .isoformat(sep=" "),
            })
            gid += 1
    return out


def gen_attendance(enrollments, all_persons):
    out = []
    aid = 1
    teacher_or_staff = [p["id"] for p in all_persons if p["type"] in ("teacher", "staff")]
    # 10 sessions per enrollment → 1200+ rows
    for e in enrollments:
        for session_idx in range(10):
            session_date = ACADEMIC_START + dt.timedelta(weeks=session_idx)
            status = weighted_choice(ATTENDANCE_STATUS_WEIGHTS)
            arrival = None
            departure = None
            late_minutes = 0
            excused = False
            excuse_reason = None
            parent_notified = False
            notified_at = None
            notes = None

            if status == "present":
                arrival = "08:00:00"
                departure = "09:30:00"
            elif status == "late":
                lm = random.randint(5, 35)
                late_minutes = lm
                arrival = f"08:{lm:02d}:00"
                departure = "09:30:00"
                if lm > 20:
                    parent_notified = True
                    notified_at = f"{session_date.isoformat()} 10:15:00"
            elif status == "absent":
                if random.random() < 0.5:
                    parent_notified = True
                    notified_at = f"{session_date.isoformat()} 10:30:00"
                    notes = "no contact from family"
            elif status == "excused":
                excused = True
                excuse_reason = random.choice(EXCUSE_REASONS)

            out.append({
                "id": aid,
                "student_id": e["student_id"],
                "course_id": e["course_id"],
                "session_date": session_date.isoformat(),
                "status": status,
                "arrival_time": arrival,
                "departure_time": departure,
                "late_minutes": late_minutes,
                "excused": excused,
                "excuse_reason": excuse_reason,
                "parent_notified": parent_notified,
                "notified_at": notified_at,
                "marked_by": random.choice(teacher_or_staff),
                "notes": notes,
                "created_at": f"{session_date.isoformat()} 10:00:00",
            })
            aid += 1
    return out


# ── SQL emitters ──────────────────────────────────────────────────────────────

def emit_persons(rows):
    cols = ("id, type, first_name, last_name, email, phone, birth_date, "
            "address, city, postal_code, active, created_at, updated_at")
    lines = [f"INSERT INTO persons ({cols}) VALUES"]
    values = []
    for p in rows:
        values.append(
            f"  ({p['id']}, {sql_str(p['type'])}, {sql_str(p['first_name'])}, "
            f"{sql_str(p['last_name'])}, {sql_str(p['email'])}, {sql_str(p['phone'])}, "
            f"{sql_str(p['birth_date'])}, {sql_str(p['address'])}, {sql_str(p['city'])}, "
            f"{sql_str(p['postal_code'])}, {sql_bool(p['active'])}, "
            f"{sql_str(p['created_at'])}, {sql_str(p['updated_at'])})"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines)


def emit_departments(rows):
    cols = "id, code, name, budget, head_teacher_id, created_at"
    lines = [f"INSERT INTO departments ({cols}) VALUES"]
    values = []
    for d in rows:
        values.append(
            f"  ({d['id']}, {sql_str(d['code'])}, {sql_str(d['name'])}, "
            f"{sql_num(d['budget'])}, {sql_num(d['head_teacher_id'])}, "
            f"{sql_str(d['created_at'])})"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines)


def emit_students(rows):
    cols = ("person_id, enrollment_year, current_grade_level, gpa, "
            "scholarship, guardian_name, guardian_phone")
    lines = [f"INSERT INTO students ({cols}) VALUES"]
    values = []
    for s in rows:
        values.append(
            f"  ({s['person_id']}, {s['enrollment_year']}, "
            f"{s['current_grade_level']}, {sql_num(s['gpa'])}, "
            f"{sql_bool(s['scholarship'])}, {sql_str(s['guardian_name'])}, "
            f"{sql_str(s['guardian_phone'])})"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines)


def emit_teachers(rows):
    cols = "person_id, hire_date, department_id, salary, tenure_status"
    lines = [f"INSERT INTO teachers ({cols}) VALUES"]
    values = []
    for t in rows:
        values.append(
            f"  ({t['person_id']}, {sql_str(t['hire_date'])}, "
            f"{sql_num(t['department_id'])}, {t['salary']}, "
            f"{sql_str(t['tenure_status'])})"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines)


def emit_staff(rows):
    cols = "person_id, hire_date, role, department_id, salary"
    lines = [f"INSERT INTO staff ({cols}) VALUES"]
    values = []
    for s in rows:
        values.append(
            f"  ({s['person_id']}, {sql_str(s['hire_date'])}, "
            f"{sql_str(s['role'])}, {sql_num(s['department_id'])}, "
            f"{s['salary']})"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines)


def emit_dept_heads(dept_heads):
    """Emit UPDATE statements to set head_teacher_id after teachers exist."""
    out = []
    for dept_id, teacher_id in dept_heads.items():
        out.append(
            f"UPDATE departments SET head_teacher_id = {teacher_id} "
            f"WHERE id = {dept_id};"
        )
    return "\n".join(out)


def emit_courses(rows):
    cols = ("id, code, name, description, credits, department_id, "
            "max_students, teacher_id")
    lines = [f"INSERT INTO courses ({cols}) VALUES"]
    values = []
    for c in rows:
        values.append(
            f"  ({c['id']}, {sql_str(c['code'])}, {sql_str(c['name'])}, "
            f"{sql_str(c['description'])}, {c['credits']}, "
            f"{c['department_id']}, {c['max_students']}, "
            f"{sql_num(c['teacher_id'])})"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines)


def emit_enrollments(rows):
    cols = "id, student_id, course_id, semester, year, status, enrolled_at"
    lines = [f"INSERT INTO enrollments ({cols}) VALUES"]
    values = []
    for e in rows:
        values.append(
            f"  ({e['id']}, {e['student_id']}, {e['course_id']}, "
            f"{sql_str(e['semester'])}, {e['year']}, "
            f"{sql_str(e['status'])}, {sql_str(e['enrolled_at'])})"
        )
    lines.append(",\n".join(values) + ";")
    return "\n".join(lines)


def emit_grades(rows):
    cols = ("id, enrollment_id, assignment_name, score, max_score, "
            "weight, graded_at")
    # Grades are chunked to keep individual statements readable
    chunks = []
    for i in range(0, len(rows), 200):
        chunk = rows[i:i+200]
        lines = [f"INSERT INTO grades ({cols}) VALUES"]
        values = []
        for g in chunk:
            values.append(
                f"  ({g['id']}, {g['enrollment_id']}, "
                f"{sql_str(g['assignment_name'])}, {g['score']}, "
                f"{g['max_score']}, {g['weight']}, {sql_str(g['graded_at'])})"
            )
        lines.append(",\n".join(values) + ";")
        chunks.append("\n".join(lines))
    return "\n\n".join(chunks)


def emit_attendance(rows):
    cols = ("id, student_id, course_id, session_date, status, arrival_time, "
            "departure_time, late_minutes, excused, excuse_reason, "
            "parent_notified, notified_at, marked_by, notes, created_at")
    chunks = []
    for i in range(0, len(rows), 250):
        chunk = rows[i:i+250]
        lines = [f"INSERT INTO attendance ({cols}) VALUES"]
        values = []
        for a in chunk:
            values.append(
                f"  ({a['id']}, {a['student_id']}, {a['course_id']}, "
                f"{sql_str(a['session_date'])}, {sql_str(a['status'])}, "
                f"{sql_str(a['arrival_time'])}, {sql_str(a['departure_time'])}, "
                f"{a['late_minutes']}, {sql_bool(a['excused'])}, "
                f"{sql_str(a['excuse_reason'])}, "
                f"{sql_bool(a['parent_notified'])}, "
                f"{sql_str(a['notified_at'])}, {sql_num(a['marked_by'])}, "
                f"{sql_str(a['notes'])}, {sql_str(a['created_at'])})"
            )
        lines.append(",\n".join(values) + ";")
        chunks.append("\n".join(lines))
    return "\n\n".join(chunks)


def main():
    gen_persons()
    departments = gen_departments()
    student_rows = gen_student_subtype()
    teacher_rows = gen_teacher_subtype()
    staff_rows   = gen_staff_subtype()
    courses      = gen_courses(teacher_rows)
    enrollments  = gen_enrollments(courses)
    grades       = gen_grades(enrollments)
    attendance   = gen_attendance(enrollments, persons)

    # Pick one head_teacher per department (first teacher we find for that dept)
    dept_heads: dict[int, int] = {}
    for t in teacher_rows:
        dept_heads.setdefault(t["department_id"], t["person_id"])

    parts = [
        "-- Generated by test-data/data/generate.py — do not edit by hand.",
        "-- Run `python3 generate.py` from this directory to regenerate.",
        f"-- Counts: persons={len(persons)} students={len(student_rows)} "
        f"teachers={len(teacher_rows)} staff={len(staff_rows)} "
        f"courses={len(courses)} enrollments={len(enrollments)} "
        f"grades={len(grades)} attendance={len(attendance)}",
        "",
        emit_persons(persons),
        "",
        emit_departments(departments),
        "",
        emit_students(student_rows),
        "",
        emit_teachers(teacher_rows),
        "",
        emit_staff(staff_rows),
        "",
        emit_dept_heads(dept_heads),
        "",
        emit_courses(courses),
        "",
        emit_enrollments(enrollments),
        "",
        emit_grades(grades),
        "",
        emit_attendance(attendance),
        "",
    ]

    OUT.write_text("\n".join(parts), encoding="utf-8")
    print(f"wrote {OUT}")
    print(f"  persons      : {len(persons)}")
    print(f"  students     : {len(student_rows)}")
    print(f"  teachers     : {len(teacher_rows)}")
    print(f"  staff        : {len(staff_rows)}")
    print(f"  departments  : {len(departments)}")
    print(f"  courses      : {len(courses)}")
    print(f"  enrollments  : {len(enrollments)}")
    print(f"  grades       : {len(grades)}")
    print(f"  attendance   : {len(attendance)}")


if __name__ == "__main__":
    main()
