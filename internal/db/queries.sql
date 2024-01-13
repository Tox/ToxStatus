-- name: GetNodeByPublicKey :many
SELECT sqlc.embed(n), sqlc.embed(a)
FROM node n
LEFT JOIN node_address a ON a.node_id = n.id
WHERE n.public_key = ?;

-- name: UpsertNode :one
INSERT INTO node(public_key, fqdn, motd)
VALUES(?, ?, ?)
ON CONFLICT(public_key) DO UPDATE SET fqdn = EXCLUDED.fqdn, motd = EXCLUDED.motd
RETURNING *;

-- name: InsertNodeAddress :one
INSERT INTO node_address(node_id, net, ip, port, ptr)
VALUES(?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateNodeAddress :one
UPDATE node_address
SET node_id = ?, net = ?, ip = ?, port = ?, ptr = ?
WHERE id = ?
RETURNING *;
