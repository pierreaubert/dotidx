-- if data are on ZFS
-- ALTER SYSTEM SET full_page_writes=off;
-- CHECKPOINT;

DO $createRolePolkadot$
BEGIN
  -- to make it easier on Ubuntu installation
  CREATE ROLE dotlake WITH LOGIN INHERIT CREATEDB;
  COMMENT ON ROLE dotlake IS 'dotlake r/w';
  EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
$createRolePolkadot$;

ALTER USER dotlake with ENCRYPTED PASSWORD 'YOURPASSWORD';

# 6 sata disks
CREATE TABLESPACE dotidx_slow0 LOCATION '/dotlake/slow0';
CREATE TABLESPACE dotidx_slow1 LOCATION '/dotlake/slow1';
CREATE TABLESPACE dotidx_slow2 LOCATION '/dotlake/slow2';
CREATE TABLESPACE dotidx_slow3 LOCATION '/dotlake/slow3';
CREATE TABLESPACE dotidx_slow4 LOCATION '/dotlake/slow4';
CREATE TABLESPACE dotidx_slow5 LOCATION '/dotlake/slow5';

ALTER TABLESPACE dotlake_slow0  OWNER TO dotlake;
ALTER TABLESPACE dotlake_slow1  OWNER TO dotlake;
ALTER TABLESPACE dotlake_slow2  OWNER TO dotlake;
ALTER TABLESPACE dotlake_slow3  OWNER TO dotlake;
ALTER TABLESPACE dotlake_slow4  OWNER TO dotlake;
ALTER TABLESPACE dotlake_slow5  OWNER TO dotlake;

# 4 ssd disks
CREATE TABLESPACE dotidx_fast0 LOCATION '/dotlake/fast0';
CREATE TABLESPACE dotidx_fast1 LOCATION '/dotlake/fast1';
CREATE TABLESPACE dotidx_fast2 LOCATION '/dotlake/fast2';
CREATE TABLESPACE dotidx_fast3 LOCATION '/dotlake/fast3';

ALTER TABLESPACE dotlake_fast0  OWNER TO dotlake;
ALTER TABLESPACE dotlake_fast1  OWNER TO dotlake;
ALTER TABLESPACE dotlake_fast2  OWNER TO dotlake;
ALTER TABLESPACE dotlake_fsat3  OWNER TO dotlake;

DO $createRoleReader$
  BEGIN
    CREATE ROLE reader WITH LOGIN;
EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
$createRoleReader$;

DO $createDB$
  BEGIN
    SELECT 'CREATE DATABASE dotlake WITH OWNER dotlake';
  EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
  END
$createDB$;





