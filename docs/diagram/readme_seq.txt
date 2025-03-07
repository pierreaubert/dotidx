sequenceDiagram
  actor Bot as Bot
  Bot ->> Idx: range or live
  Sidecar ->> Node: get raw blocks
  Node ->> RocksDB: read k
  RocksDB ->> Node: various v
  Node ->> Sidecar: raw blocks
  Sidecar ->> Sidecar: decode blocks
  Idx ->> Idx: parse blocks
  Idx ->> SQL DB: add blocks and mapping
