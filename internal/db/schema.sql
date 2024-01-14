CREATE TABLE IF NOT EXISTS node (
  id            INTEGER NOT NULL PRIMARY KEY,
  created_at    REAL NOT NULL DEFAULT(unixepoch('subsec')),
  -- The last time this node's public key was seen in the DHT
  last_seen_at  REAL NOT NULL DEFAULT(unixepoch('subsec')),
  public_key    TEXT NOT NULL UNIQUE CHECK (LENGTH(public_key) == 64),
  fqdn          TEXT,
  motd          TEXT
) STRICT;

CREATE TABLE IF NOT EXISTS node_address (
  id            INTEGER NOT NULL PRIMARY KEY,
  created_at    REAL NOT NULL DEFAULT(unixepoch('subsec')),
  -- The last time this node address was seen in the DHT
  last_seen_at  REAL NOT NULL DEFAULT(unixepoch('subsec')),
  -- The last time we pinged this node address with a getnodes request
  last_ping_at  REAL,
  -- The last time we received a response from this node address to our getnodes request
  last_pong_at  REAL,
  node_id       INTEGER NOT NULL,
  net           TEXT NOT NULL CHECK (net IN ('udp4', 'udp6', 'tcp4', 'tcp6')),
  ip            TEXT NOT NULL,
  port          INTEGER NOT NULL CHECK (port > 0 AND port < 65536),
  ptr           TEXT,
  UNIQUE(node_id, net, ip, port),
  FOREIGN KEY (node_id) REFERENCES node (id) 
) STRICT;
