# Supabase Atlas Migration Runbook

## Overview
This document records the exact steps, commands, and fixes used to push the Ent schema for the Paycrest Aggregator into a Supabase Postgres database via Atlas migrations.

## Environment
- **Project root**: `/home/commendatore/Desktop/NEDA/rails/aggregator`
- **CLI tools**: [`atlas`](https://atlasgo.io/), `psql`
- **Database**: Supabase pooler endpoint (`aws-1-eu-central-1.pooler.supabase.com:6543`)
- **SSL**: Required

## Prerequisites
- Ensure Atlas CLI is installed (see `install_atlas.sh`).
- Install a specific Postgres client to get `psql`:
  ```bash
  sudo apt install postgresql-client-16
  ```

## Connection Setup
1. Export the Supabase pooler DSN (replace placeholders with your credentials):
   ```bash
   export DATABASE_URL="postgresql://<DB_USER>.<PROJECT_REF>:<DB_PASSWORD>@aws-1-eu-central-1.pooler.supabase.com:6543/<DB_NAME>?sslmode=require"
   ```
2. Verify connectivity:
   ```bash
   psql "$DATABASE_URL" -c '\conninfo'
   ```

## Prepare Atlas Revision Schema
Atlas stores migration history in `atlas_schema_revisions.atlas_schema_revisions`.

1. Create the schema (idempotent):
   ```bash
   psql "$DATABASE_URL" -c 'CREATE SCHEMA IF NOT EXISTS atlas_schema_revisions;'
   ```
2. Remove any conflicting table in `public` (optional clean-up):
   ```bash
   psql "$DATABASE_URL" -c 'DROP TABLE IF EXISTS public.atlas_schema_revisions;'
   ```
3. Create the revision table inside the schema:
   ```bash
   psql "$DATABASE_URL" <<'SQL'
   CREATE TABLE IF NOT EXISTS atlas_schema_revisions.atlas_schema_revisions (
       version          varchar PRIMARY KEY,
       description      varchar NOT NULL,
       type             bigint NOT NULL DEFAULT 2,
       applied          bigint NOT NULL DEFAULT 0,
       total            bigint NOT NULL DEFAULT 0,
       executed_at      timestamptz NOT NULL,
       execution_time   bigint NOT NULL,
       error            text,
       error_stmt       text,
       hash             varchar NOT NULL,
       partial_hashes   jsonb,
       operator_version varchar NOT NULL
   );
   SQL
   ```

## Apply Atlas Migrations
Supabase creates system schemas (e.g. `auth`, `storage`), so Atlas must be allowed to run on a "dirty" database and reuse the existing revisions schema.

```bash
atlas migrate apply \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --allow-dirty \
  --revisions-schema atlas_schema_revisions
```

> This command successfully ran 64 migrations (~7m39s) and executed 259 SQL statements when applied on 2025-10-02.

## Verify Migration Status
Once the apply completes, confirm the database state:
```bash
atlas migrate status \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --revisions-schema atlas_schema_revisions
```

## Common Errors & Fixes
- **`dial tcp ... connect: connection refused`**
  - Cause: Using the non-pooler host/port or SSL disabled.
  - Fix: Use the pooler DSN on port `6543` with `sslmode=require`.
- **`relation "atlas_schema_revisions" already exists`**
  - Cause: Table in `public` conflicts with Atlas-managed table.
  - Fix: Drop the `public.atlas_schema_revisions` table.
- **`connected database is not clean: found schema "auth"`**
  - Cause: Supabase system schemas detected.
  - Fix: Pass `--allow-dirty` when running Atlas commands.
- **`We couldn't find a revision table...`**
  - Cause: Atlas found `atlas_schema_revisions` schema but wasn't told to use it.
  - Fix: Add `--revisions-schema atlas_schema_revisions`.
- **`Command 'psql' not found`**
  - Fix: Install a versioned client, e.g. `sudo apt install postgresql-client-16`.

## Quick Reference Checklist
1. Export pooler `DATABASE_URL` (SSL required).
2. Verify DB connectivity via `psql`.
3. Ensure `atlas_schema_revisions` schema/table exist.
4. Run `atlas migrate apply --allow-dirty --revisions-schema atlas_schema_revisions`.
5. Check `atlas migrate status` to confirm revision state.
