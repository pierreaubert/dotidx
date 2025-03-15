-- if data are on ZFS
ALTER SYSTEM SET full_page_writes=off;
CHECKPOINT;

DO $createRolePolkadot$
BEGIN
  -- to make it easier on Ubuntu installation
  CREATE ROLE dotidx WITH LOGIN INHERIT CREATEDB;
  COMMENT ON ROLE dotidx IS 'dotidx r/w';
  EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
$createRolePolkadot$;

ALTER USER dotidx with ENCRYPTED PASSWORD 'YOURPASSWORD';

--  6 sata disks
CREATE TABLESPACE dotidx_slow0 LOCATION '/dotidx/slow0';
CREATE TABLESPACE dotidx_slow1 LOCATION '/dotidx/slow1';
CREATE TABLESPACE dotidx_slow2 LOCATION '/dotidx/slow2';
CREATE TABLESPACE dotidx_slow3 LOCATION '/dotidx/slow3';
CREATE TABLESPACE dotidx_slow4 LOCATION '/dotidx/slow4';
CREATE TABLESPACE dotidx_slow5 LOCATION '/dotidx/slow5';

ALTER TABLESPACE dotidx_slow0  OWNER TO dotidx;
ALTER TABLESPACE dotidx_slow1  OWNER TO dotidx;
ALTER TABLESPACE dotidx_slow2  OWNER TO dotidx;
ALTER TABLESPACE dotidx_slow3  OWNER TO dotidx;
ALTER TABLESPACE dotidx_slow4  OWNER TO dotidx;
ALTER TABLESPACE dotidx_slow5  OWNER TO dotidx;

--  4 ssd disks
CREATE TABLESPACE dotidx_fast0 LOCATION '/dotidx/fast0';
CREATE TABLESPACE dotidx_fast1 LOCATION '/dotidx/fast1';
CREATE TABLESPACE dotidx_fast2 LOCATION '/dotidx/fast2';
CREATE TABLESPACE dotidx_fast3 LOCATION '/dotidx/fast3';

ALTER TABLESPACE dotidx_fast0  OWNER TO dotidx;
ALTER TABLESPACE dotidx_fast1  OWNER TO dotidx;
ALTER TABLESPACE dotidx_fast2  OWNER TO dotidx;
ALTER TABLESPACE dotidx_fast3  OWNER TO dotidx;

DO $createRoleReader$
  BEGIN
    CREATE ROLE reader WITH LOGIN;
EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
$createRoleReader$;






