package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Tox/ToxStatus/internal/db"
	"github.com/Tox/ToxStatus/internal/models"
	"github.com/alexbakker/tox4go/dht"
)

var ErrNotFound = fmt.Errorf("not found: %w", sql.ErrNoRows)

type NodesRepo struct {
	db *sql.DB
	q  *db.Queries
}

func New(sqldb *sql.DB) *NodesRepo {
	return &NodesRepo{
		db: sqldb,
		q:  db.New(sqldb),
	}
}

func (r *NodesRepo) GetNodeByPublicKey(ctx context.Context, pk *dht.PublicKey) (*models.Node, error) {
	rows, err := r.q.GetNodeByPublicKey(ctx, (*db.PublicKey)(pk))
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, ErrNotFound
	}

	node := convertNode(&rows[0].Node)
	for _, row := range rows {
		node.Addresses = append(node.Addresses, &models.NodeAddress{
			ID:         row.NodeAddress.ID,
			CreatedAt:  time.Time(row.NodeAddress.CreatedAt),
			LastSeenAt: time.Time(row.NodeAddress.LastSeenAt),
			LastPingAt: time.Time(row.NodeAddress.LastPingAt),
			LastPongAt: time.Time(row.NodeAddress.LastPongAt),
			Net:        row.NodeAddress.Net,
			IP:         row.NodeAddress.Ip,
			Port:       int(row.NodeAddress.Port),
			Ptr:        convertNullString(row.NodeAddress.Ptr),
		})
	}

	return node, nil
}

func (r *NodesRepo) UpsertNode(ctx context.Context, node *models.Node) (*models.Node, error) {
	dbNode, err := r.q.UpsertNode(ctx, &db.UpsertNodeParams{
		PublicKey: (*db.PublicKey)(node.PublicKey),
		Fqdn:      newNullString(node.FQDN),
		Motd:      newNullString(node.MOTD),
	})
	if err != nil {
		return nil, err
	}

	resNode := convertNode(dbNode)
	resNode.Addresses = node.Addresses
	return resNode, nil
}

func (r *NodesRepo) TrackDHTNode(ctx context.Context, node *dht.Node) (*models.Node, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	q := r.q.WithTx(tx)
	dbNode, err := q.UpsertNode(ctx, &db.UpsertNodeParams{
		PublicKey: (*db.PublicKey)(node.PublicKey),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert node: %w", err)
	}

	dbNodeAddr, err := q.InsertNodeAddress(ctx, &db.InsertNodeAddressParams{
		NodeID: dbNode.ID,
		Net:    node.Type.Net(),
		Ip:     node.IP.String(),
		Port:   int64(node.Port),
	})
	if err != nil {
		return nil, fmt.Errorf("insert node address: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	res := convertNode(dbNode)
	nodeAddr := convertNodeAddress(dbNodeAddr)
	res.Addresses = append(res.Addresses, nodeAddr)
	return res, nil
}

func convertNode(dbNode *db.Node) *models.Node {
	return &models.Node{
		ID:         dbNode.ID,
		CreatedAt:  time.Time(dbNode.CreatedAt),
		LastSeenAt: time.Time(dbNode.LastSeenAt),
		PublicKey:  (*dht.PublicKey)(dbNode.PublicKey),
		FQDN:       convertNullString(dbNode.Fqdn),
		MOTD:       convertNullString(dbNode.Motd),
	}
}

func convertNodeAddress(dbNodeAddr *db.NodeAddress) *models.NodeAddress {
	return &models.NodeAddress{
		ID:         dbNodeAddr.ID,
		CreatedAt:  time.Time(dbNodeAddr.CreatedAt),
		LastSeenAt: time.Time(dbNodeAddr.LastSeenAt),
		LastPingAt: time.Time(dbNodeAddr.LastPingAt),
		LastPongAt: time.Time(dbNodeAddr.LastPongAt),
		//NodeID:     dbNodeAddr.NodeID,
		Net:  dbNodeAddr.Net,
		IP:   dbNodeAddr.Ip,
		Port: int(dbNodeAddr.Port),
		Ptr:  convertNullString(dbNodeAddr.Ptr),
	}
}

func convertNullString(s sql.NullString) *string {
	if s.Valid {
		return &s.String
	}
	return nil
}

func newNullString(s *string) sql.NullString {
	res := sql.NullString{Valid: s != nil}
	if res.Valid {
		res.String = *s
	}
	return res
}
