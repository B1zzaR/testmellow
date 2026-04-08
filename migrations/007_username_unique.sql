-- Migration 007: enforce username uniqueness at the DB level and add index for lookups
-- Safe to run on existing data; fails if duplicates already exist (none expected in production).

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username)
    WHERE username IS NOT NULL;
