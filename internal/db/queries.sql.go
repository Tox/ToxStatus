// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: queries.sql

package db

import (
	"context"
	"database/sql"
)

const getNodeAddress = `-- name: GetNodeAddress :one
SELECT a.id
FROM node_address a
JOIN node n ON n.id = a.node_id
WHERE n.public_key = ? AND a.net = ? AND a.ip = ? AND a.port = ?
`

type GetNodeAddressParams struct {
	PublicKey *PublicKey
	Net       string
	Ip        string
	Port      int64
}

func (q *Queries) GetNodeAddress(ctx context.Context, arg *GetNodeAddressParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, getNodeAddress,
		arg.PublicKey,
		arg.Net,
		arg.Ip,
		arg.Port,
	)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const getNodeByPublicKey = `-- name: GetNodeByPublicKey :many
SELECT n.id, n.created_at, n.last_seen_at, n.public_key, n.fqdn, n.motd, a.id, a.created_at, a.last_seen_at, a.last_ping_at, a.last_pong_at, a.node_id, a.net, a.ip, a.port, a.ptr
FROM node n
JOIN node_address a ON a.node_id = n.id
WHERE n.public_key = ?
`

type GetNodeByPublicKeyRow struct {
	Node        Node
	NodeAddress NodeAddress
}

func (q *Queries) GetNodeByPublicKey(ctx context.Context, publicKey *PublicKey) ([]*GetNodeByPublicKeyRow, error) {
	rows, err := q.db.QueryContext(ctx, getNodeByPublicKey, publicKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetNodeByPublicKeyRow
	for rows.Next() {
		var i GetNodeByPublicKeyRow
		if err := rows.Scan(
			&i.Node.ID,
			&i.Node.CreatedAt,
			&i.Node.LastSeenAt,
			&i.Node.PublicKey,
			&i.Node.Fqdn,
			&i.Node.Motd,
			&i.NodeAddress.ID,
			&i.NodeAddress.CreatedAt,
			&i.NodeAddress.LastSeenAt,
			&i.NodeAddress.LastPingAt,
			&i.NodeAddress.LastPongAt,
			&i.NodeAddress.NodeID,
			&i.NodeAddress.Net,
			&i.NodeAddress.Ip,
			&i.NodeAddress.Port,
			&i.NodeAddress.Ptr,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getNodeCount = `-- name: GetNodeCount :one
SELECT COUNT(*)
FROM node
`

func (q *Queries) GetNodeCount(ctx context.Context) (int64, error) {
	row := q.db.QueryRowContext(ctx, getNodeCount)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getResponsiveNodes = `-- name: GetResponsiveNodes :many
SELECT n.id, n.created_at, n.last_seen_at, n.public_key, n.fqdn, n.motd, a.id, a.created_at, a.last_seen_at, a.last_ping_at, a.last_pong_at, a.node_id, a.net, a.ip, a.port, a.ptr
FROM node n
JOIN node_address a ON a.node_id = n.id
WHERE a.last_pong_at IS NOT NULL
`

type GetResponsiveNodesRow struct {
	Node        Node
	NodeAddress NodeAddress
}

func (q *Queries) GetResponsiveNodes(ctx context.Context) ([]*GetResponsiveNodesRow, error) {
	rows, err := q.db.QueryContext(ctx, getResponsiveNodes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetResponsiveNodesRow
	for rows.Next() {
		var i GetResponsiveNodesRow
		if err := rows.Scan(
			&i.Node.ID,
			&i.Node.CreatedAt,
			&i.Node.LastSeenAt,
			&i.Node.PublicKey,
			&i.Node.Fqdn,
			&i.Node.Motd,
			&i.NodeAddress.ID,
			&i.NodeAddress.CreatedAt,
			&i.NodeAddress.LastSeenAt,
			&i.NodeAddress.LastPingAt,
			&i.NodeAddress.LastPongAt,
			&i.NodeAddress.NodeID,
			&i.NodeAddress.Net,
			&i.NodeAddress.Ip,
			&i.NodeAddress.Port,
			&i.NodeAddress.Ptr,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getUnresponsiveNodes = `-- name: GetUnresponsiveNodes :many
SELECT n.id, n.created_at, n.last_seen_at, n.public_key, n.fqdn, n.motd, a.id, a.created_at, a.last_seen_at, a.last_ping_at, a.last_pong_at, a.node_id, a.net, a.ip, a.port, a.ptr
FROM node n
JOIN node_address a ON a.node_id = n.id
WHERE a.last_pong_at IS NULL
  AND a.last_ping_at IS NOT NULL
  AND (unixepoch('subsec') - a.last_ping_at) >= CAST(?1 AS REAL)
`

type GetUnresponsiveNodesRow struct {
	Node        Node
	NodeAddress NodeAddress
}

func (q *Queries) GetUnresponsiveNodes(ctx context.Context, retryDelay float64) ([]*GetUnresponsiveNodesRow, error) {
	rows, err := q.db.QueryContext(ctx, getUnresponsiveNodes, retryDelay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetUnresponsiveNodesRow
	for rows.Next() {
		var i GetUnresponsiveNodesRow
		if err := rows.Scan(
			&i.Node.ID,
			&i.Node.CreatedAt,
			&i.Node.LastSeenAt,
			&i.Node.PublicKey,
			&i.Node.Fqdn,
			&i.Node.Motd,
			&i.NodeAddress.ID,
			&i.NodeAddress.CreatedAt,
			&i.NodeAddress.LastSeenAt,
			&i.NodeAddress.LastPingAt,
			&i.NodeAddress.LastPongAt,
			&i.NodeAddress.NodeID,
			&i.NodeAddress.Net,
			&i.NodeAddress.Ip,
			&i.NodeAddress.Port,
			&i.NodeAddress.Ptr,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const hasNodeByPublicKey = `-- name: HasNodeByPublicKey :one
SELECT EXISTS(
  SELECT 1
  FROM node
  WHERE public_key = ?
)
`

func (q *Queries) HasNodeByPublicKey(ctx context.Context, publicKey *PublicKey) (int64, error) {
	row := q.db.QueryRowContext(ctx, hasNodeByPublicKey, publicKey)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

const pingNodeAddress = `-- name: PingNodeAddress :exec
UPDATE node_address
SET last_ping_at = unixepoch('subsec')
WHERE id = ?
`

func (q *Queries) PingNodeAddress(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, pingNodeAddress, id)
	return err
}

const pongNodeAddress = `-- name: PongNodeAddress :exec
UPDATE node_address
SET last_pong_at = unixepoch('subsec')
WHERE id = ?
`

func (q *Queries) PongNodeAddress(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, pongNodeAddress, id)
	return err
}

const updateNodeAddress = `-- name: UpdateNodeAddress :one
UPDATE node_address
SET node_id = ?, net = ?, ip = ?, port = ?, ptr = ?
WHERE id = ?
RETURNING id, created_at, last_seen_at, last_ping_at, last_pong_at, node_id, net, ip, port, ptr
`

type UpdateNodeAddressParams struct {
	NodeID int64
	Net    string
	Ip     string
	Port   int64
	Ptr    sql.NullString
	ID     int64
}

func (q *Queries) UpdateNodeAddress(ctx context.Context, arg *UpdateNodeAddressParams) (*NodeAddress, error) {
	row := q.db.QueryRowContext(ctx, updateNodeAddress,
		arg.NodeID,
		arg.Net,
		arg.Ip,
		arg.Port,
		arg.Ptr,
		arg.ID,
	)
	var i NodeAddress
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.LastSeenAt,
		&i.LastPingAt,
		&i.LastPongAt,
		&i.NodeID,
		&i.Net,
		&i.Ip,
		&i.Port,
		&i.Ptr,
	)
	return &i, err
}

const upsertNode = `-- name: UpsertNode :one
INSERT INTO node(public_key, fqdn, motd)
VALUES(?, ?, ?)
ON CONFLICT(public_key) DO UPDATE SET fqdn = EXCLUDED.fqdn, motd = EXCLUDED.motd, last_seen_at = unixepoch('subsec')
RETURNING id, created_at, last_seen_at, public_key, fqdn, motd
`

type UpsertNodeParams struct {
	PublicKey *PublicKey
	Fqdn      sql.NullString
	Motd      sql.NullString
}

func (q *Queries) UpsertNode(ctx context.Context, arg *UpsertNodeParams) (*Node, error) {
	row := q.db.QueryRowContext(ctx, upsertNode, arg.PublicKey, arg.Fqdn, arg.Motd)
	var i Node
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.LastSeenAt,
		&i.PublicKey,
		&i.Fqdn,
		&i.Motd,
	)
	return &i, err
}

const upsertNodeAddress = `-- name: UpsertNodeAddress :one
INSERT INTO node_address(node_id, net, ip, port, ptr)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(node_id, net, ip, port) DO UPDATE SET last_seen_at = unixepoch('subsec')
RETURNING id, created_at, last_seen_at, last_ping_at, last_pong_at, node_id, net, ip, port, ptr
`

type UpsertNodeAddressParams struct {
	NodeID int64
	Net    string
	Ip     string
	Port   int64
	Ptr    sql.NullString
}

func (q *Queries) UpsertNodeAddress(ctx context.Context, arg *UpsertNodeAddressParams) (*NodeAddress, error) {
	row := q.db.QueryRowContext(ctx, upsertNodeAddress,
		arg.NodeID,
		arg.Net,
		arg.Ip,
		arg.Port,
		arg.Ptr,
	)
	var i NodeAddress
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.LastSeenAt,
		&i.LastPingAt,
		&i.LastPongAt,
		&i.NodeID,
		&i.Net,
		&i.Ip,
		&i.Port,
		&i.Ptr,
	)
	return &i, err
}
