-- Data-model field correction (grounded in Frappe Education / openSIS / OpenEduCat / Gibbon).
-- All ADDITIVE (nullable or defaulted) — nothing existing breaks. Legacy denormalised columns
-- (students.course_name/group_name/mentor_name/telegram_id) are kept for now, retired later.

-- students: real CRM contact/identity/lifecycle fields + a proper mentor FK + parent contact.
ALTER TABLE "students" ADD COLUMN "email" text NULL;
ALTER TABLE "students" ADD COLUMN "birth_date" date NULL;
ALTER TABLE "students" ADD COLUMN "gender" text NOT NULL DEFAULT '';
ALTER TABLE "students" ADD COLUMN "second_phone" text NULL;
ALTER TABLE "students" ADD COLUMN "address" text NULL;
ALTER TABLE "students" ADD COLUMN "parent_name" text NULL;
ALTER TABLE "students" ADD COLUMN "parent_phone" text NULL;
ALTER TABLE "students" ADD COLUMN "student_code" text NULL;
ALTER TABLE "students" ADD COLUMN "status" text NOT NULL DEFAULT 'active';
ALTER TABLE "students" ADD COLUMN "mentor_id" uuid NULL;
ALTER TABLE "students" ADD CONSTRAINT "students_mentor_id_fkey" FOREIGN KEY ("mentor_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;

-- groups: link to the course the group runs + capacity + run dates.
ALTER TABLE "groups" ADD COLUMN "course_id" uuid NULL;
ALTER TABLE "groups" ADD COLUMN "capacity" integer NOT NULL DEFAULT 0;
ALTER TABLE "groups" ADD COLUMN "start_date" date NULL;
ALTER TABLE "groups" ADD COLUMN "end_date" date NULL;
ALTER TABLE "groups" ADD CONSTRAINT "groups_course_id_fkey" FOREIGN KEY ("course_id") REFERENCES "courses" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;

-- courses: short human code (e.g. "IELTS-PRE").
ALTER TABLE "courses" ADD COLUMN "code" text NOT NULL DEFAULT '';

-- users: contact phone + deactivate-without-delete flag.
ALTER TABLE "users" ADD COLUMN "phone" text NULL;
ALTER TABLE "users" ADD COLUMN "is_active" boolean NOT NULL DEFAULT true;
