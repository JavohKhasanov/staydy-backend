-- Per-center gamification settings (tunable from the admin panel). Coins are derived from XP by a
-- level-scaled rate: coins = xp * (coin_base + (level-1) * coin_step); level = xp / level_size + 1.
ALTER TABLE "organizations" ADD COLUMN "gx_xp_attend"        int NOT NULL DEFAULT 2;  -- on-time attendance
ALTER TABLE "organizations" ADD COLUMN "gx_xp_late"          int NOT NULL DEFAULT 1;  -- late attendance
ALTER TABLE "organizations" ADD COLUMN "gx_xp_homework_max"  int NOT NULL DEFAULT 5;  -- teacher grades 0..this
ALTER TABLE "organizations" ADD COLUMN "gx_xp_checkin"       int NOT NULL DEFAULT 2;  -- weekly check-in
ALTER TABLE "organizations" ADD COLUMN "gx_level_size"       int NOT NULL DEFAULT 300; -- XP per level
ALTER TABLE "organizations" ADD COLUMN "gx_coin_base"        int NOT NULL DEFAULT 4;  -- coins per XP at level 1
ALTER TABLE "organizations" ADD COLUMN "gx_coin_step"        int NOT NULL DEFAULT 1;  -- +coins per XP per level above 1
