-- pure laziness -> should be automated if i keep the gin index
-- 2020

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_05_gin_idx ON chain.blocks_polkadot_polkadot_2020_05 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_06_gin_idx ON chain.blocks_polkadot_polkadot_2020_06 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_07_gin_idx ON chain.blocks_polkadot_polkadot_2020_07 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_08_gin_idx  ON chain.blocks_polkadot_polkadot_2020_08 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_09_gin_idx  ON chain.blocks_polkadot_polkadot_2020_09 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_10_gin_idx ON chain.blocks_polkadot_polkadot_2020_10 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_11_gin_idx  ON chain.blocks_polkadot_polkadot_2020_11 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2020_12_gin_idx  ON chain.blocks_polkadot_polkadot_2020_12 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

-- 2021

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_01_gin_idx ON chain.blocks_polkadot_polkadot_2021_01 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_02_gin_idx ON chain.blocks_polkadot_polkadot_2021_02 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_03_gin_idx ON chain.blocks_polkadot_polkadot_2021_03 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_04_gin_idx ON chain.blocks_polkadot_polkadot_2021_04 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_05_gin_idx ON chain.blocks_polkadot_polkadot_2021_05 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_06_gin_idx ON chain.blocks_polkadot_polkadot_2021_06 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_07_gin_idx ON chain.blocks_polkadot_polkadot_2021_07 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_08_gin_idx  ON chain.blocks_polkadot_polkadot_2021_08 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_09_gin_idx  ON chain.blocks_polkadot_polkadot_2021_09 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_10_gin_idx ON chain.blocks_polkadot_polkadot_2021_10 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_11_gin_idx  ON chain.blocks_polkadot_polkadot_2021_11 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2021_12_gin_idx  ON chain.blocks_polkadot_polkadot_2021_12 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

-- 2022

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_01_gin_idx ON chain.blocks_polkadot_polkadot_2022_01 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_02_gin_idx ON chain.blocks_polkadot_polkadot_2022_02 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_03_gin_idx ON chain.blocks_polkadot_polkadot_2022_03 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_04_gin_idx ON chain.blocks_polkadot_polkadot_2022_04 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_05_gin_idx ON chain.blocks_polkadot_polkadot_2022_05 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_06_gin_idx ON chain.blocks_polkadot_polkadot_2022_06 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_07_gin_idx ON chain.blocks_polkadot_polkadot_2022_07 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_08_gin_idx  ON chain.blocks_polkadot_polkadot_2022_08 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_09_gin_idx  ON chain.blocks_polkadot_polkadot_2022_09 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_10_gin_idx ON chain.blocks_polkadot_polkadot_2022_10 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_11_gin_idx  ON chain.blocks_polkadot_polkadot_2022_11 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2022_12_gin_idx  ON chain.blocks_polkadot_polkadot_2022_12 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

-- 2023

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_01_gin_idx ON chain.blocks_polkadot_polkadot_2023_01 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_02_gin_idx ON chain.blocks_polkadot_polkadot_2023_02 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_03_gin_idx ON chain.blocks_polkadot_polkadot_2023_03 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_04_gin_idx ON chain.blocks_polkadot_polkadot_2023_04 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_05_gin_idx ON chain.blocks_polkadot_polkadot_2023_05 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_06_gin_idx ON chain.blocks_polkadot_polkadot_2023_06 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_07_gin_idx ON chain.blocks_polkadot_polkadot_2023_07 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_08_gin_idx  ON chain.blocks_polkadot_polkadot_2023_08 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_09_gin_idx  ON chain.blocks_polkadot_polkadot_2023_09 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_10_gin_idx ON chain.blocks_polkadot_polkadot_2023_10 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_11_gin_idx  ON chain.blocks_polkadot_polkadot_2023_11 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2023_12_gin_idx  ON chain.blocks_polkadot_polkadot_2023_12 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;


-- 2024

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_01_gin_idx ON chain.blocks_polkadot_polkadot_2024_01 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_02_gin_idx ON chain.blocks_polkadot_polkadot_2024_02 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_03_gin_idx ON chain.blocks_polkadot_polkadot_2024_03 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_04_gin_idx ON chain.blocks_polkadot_polkadot_2024_04 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_05_gin_idx ON chain.blocks_polkadot_polkadot_2024_05 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_06_gin_idx ON chain.blocks_polkadot_polkadot_2024_06 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_07_gin_idx ON chain.blocks_polkadot_polkadot_2024_07 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_08_gin_idx  ON chain.blocks_polkadot_polkadot_2024_08 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_09_gin_idx  ON chain.blocks_polkadot_polkadot_2024_09 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_10_gin_idx ON chain.blocks_polkadot_polkadot_2024_10 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_11_gin_idx  ON chain.blocks_polkadot_polkadot_2024_11 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2024_12_gin_idx  ON chain.blocks_polkadot_polkadot_2024_12 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

-- 2025

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_01_gin_idx ON chain.blocks_polkadot_polkadot_2025_01 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_02_gin_idx ON chain.blocks_polkadot_polkadot_2025_02 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_03_gin_idx ON chain.blocks_polkadot_polkadot_2025_03 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_04_gin_idx ON chain.blocks_polkadot_polkadot_2025_04 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_05_gin_idx ON chain.blocks_polkadot_polkadot_2025_05 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_06_gin_idx ON chain.blocks_polkadot_polkadot_2025_06 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_07_gin_idx ON chain.blocks_polkadot_polkadot_2025_07 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_08_gin_idx  ON chain.blocks_polkadot_polkadot_2025_08 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_09_gin_idx  ON chain.blocks_polkadot_polkadot_2025_09 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast0;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_10_gin_idx ON chain.blocks_polkadot_polkadot_2025_10 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast1;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_11_gin_idx  ON chain.blocks_polkadot_polkadot_2025_11 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast2;

CREATE INDEX IF NOT EXISTS blocks_polkadot_polkadot_2025_12_gin_idx  ON chain.blocks_polkadot_polkadot_2025_12 USING gin
  (extrinsics jsonb_path_ops) WITH (fastupdate=False) TABLESPACE dotidx_fast3;

