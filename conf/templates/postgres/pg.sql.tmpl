-- if data are on ZFS
ALTER SYSTEM SET full_page_writes=off;
CHECKPOINT;

DO $createRolePolkadot$
BEGIN
  -- to make it easier on Ubuntu installation
  CREATE ROLE {{.DotidxDB.User}} WITH LOGIN INHERIT CREATEDB;
  COMMENT ON ROLE {{.DotidxDB.User}} IS '{{.DotidxDB.Name}} r/w';
  EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
$createRolePolkadot$;

ALTER USER {{.DotidxDB.User}} with ENCRYPTED PASSWORD 'YOURPASSWORD';

--  6 sata disks
CREATE TABLESPACE dotidx_slow0 LOCATION '{{.DotidxRoot}}/slow0';
CREATE TABLESPACE dotidx_slow1 LOCATION '{{.DotidxRoot}}/slow1';
CREATE TABLESPACE dotidx_slow2 LOCATION '{{.DotidxRoot}}/slow2';
CREATE TABLESPACE dotidx_slow3 LOCATION '{{.DotidxRoot}}/slow3';
CREATE TABLESPACE dotidx_slow4 LOCATION '{{.DotidxRoot}}/slow4';
CREATE TABLESPACE dotidx_slow5 LOCATION '{{.DotidxRoot}}/slow5';

ALTER TABLESPACE dotidx_slow0  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_slow1  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_slow2  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_slow3  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_slow4  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_slow5  OWNER TO {{.DotidxDB.User}};

--  4 ssd disks
CREATE TABLESPACE dotidx_fast0 LOCATION '{{.DotidxRoot}}/fast0';
CREATE TABLESPACE dotidx_fast1 LOCATION '{{.DotidxRoot}}/fast1';
CREATE TABLESPACE dotidx_fast2 LOCATION '{{.DotidxRoot}}/fast2';
CREATE TABLESPACE dotidx_fast3 LOCATION '{{.DotidxRoot}}/fast3';

ALTER TABLESPACE dotidx_fast0  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_fast1  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_fast2  OWNER TO {{.DotidxDB.User}};
ALTER TABLESPACE dotidx_fast3  OWNER TO {{.DotidxDB.User}};

DO $createRoleReader$
  BEGIN
    CREATE ROLE reader WITH LOGIN;
EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
$createRoleReader$;

CREATE SCHEMA chain;
ALTER SCHEMA chain OWNER TO {{.DotidxDB.User}};






