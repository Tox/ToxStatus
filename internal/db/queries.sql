-- name: GetNodeByPublicKey :many
SELECT sqlc.embed(n), sqlc.embed(a)
FROM node n
JOIN node_address a ON a.node_id = n.id
WHERE n.public_key = ?;

-- name: HasNodeByPublicKey :one
SELECT EXISTS(
  SELECT 1
  FROM node
  WHERE public_key = ?
);

-- name: GetNodeCount :one
SELECT COUNT(*)
FROM node;

-- name: UpsertNode :one
INSERT INTO node(public_key, fqdn, motd)
VALUES(?, ?, ?)
ON CONFLICT(public_key) DO UPDATE SET fqdn = EXCLUDED.fqdn, motd = EXCLUDED.motd, last_seen_at = unixepoch('subsec')
RETURNING *;

-- name: UpdateNodeInfo :exec
UPDATE node
SET motd = ?, version = ?, last_seen_at = unixepoch('subsec')
WHERE public_key = ?;

-- name: UpsertNodeAddress :one
INSERT INTO node_address(node_id, net, ip, port, ptr)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(node_id, net, ip, port) DO UPDATE SET last_seen_at = unixepoch('subsec')
RETURNING *;

-- name: UpdateNodeAddress :one
UPDATE node_address
SET node_id = ?, net = ?, ip = ?, port = ?, ptr = ?
WHERE id = ?
RETURNING *;

-- name: GetNodeAddress :one
SELECT a.id
FROM node_address a
JOIN node n ON n.id = a.node_id
WHERE n.public_key = ? AND a.net = ? AND a.ip = ? AND a.port = ?;

-- name: PingNodeAddress :exec
UPDATE node_address
SET last_ping_at = unixepoch('subsec')
WHERE id = ?;

-- name: PongNodeAddress :exec
UPDATE node_address
SET last_pong_at = unixepoch('subsec')
WHERE id = ?;

-- name: GetResponsiveNodes :many
SELECT sqlc.embed(n), sqlc.embed(a)
FROM node n
JOIN node_address a ON a.node_id = n.id
WHERE a.last_pong_at IS NOT NULL;

-- name: GetUnresponsiveNodes :many
SELECT sqlc.embed(n), sqlc.embed(a)
FROM node n
JOIN node_address a ON a.node_id = n.id
WHERE a.last_pong_at IS NULL
  AND a.last_ping_at IS NOT NULL
  AND (unixepoch('subsec') - a.last_ping_at) >= CAST(sqlc.arg(retry_delay) AS REAL);

-- name: GetNodeByAddress :one
SELECT sqlc.embed(n)
FROM node n
JOIN node_address a ON a.node_id = n.id
WHERE a.net = ? AND a.ip = ? AND a.port = ?;
