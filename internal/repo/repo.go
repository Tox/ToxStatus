package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Tox/ToxStatus/internal/db"
	"github.com/Tox/ToxStatus/internal/models"
	"github.com/alexbakker/tox4go/dht"
	"golang.org/x/exp/maps"
)

var ErrNotFound = fmt.Errorf("not found: %w", sql.ErrNoRows)

type NodesRepo struct {
	db *sql.DB
	q  *db.Queries
}

type nodeAddressCombo struct {
	Node        db.Node
	NodeAddress db.NodeAddress
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

func (r *NodesRepo) HasNodeByPublicKey(ctx context.Context, pk *dht.PublicKey) (bool, error) {
	res, err := r.q.HasNodeByPublicKey(ctx, (*db.PublicKey)(pk))
	if err != nil {
		return false, err
	}

	return res == 1, nil
}

func (r *NodesRepo) GetNodeCount(ctx context.Context) (int64, error) {
	return r.q.GetNodeCount(ctx)
}

/*func (r *NodesRepo) UpsertNode(ctx context.Context, node *models.Node) (*models.Node, error) {
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
}*/

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

	dbNodeAddr, err := q.UpsertNodeAddress(ctx, &db.UpsertNodeAddressParams{
		NodeID: dbNode.ID,
		Net:    node.Type.Net(),
		Ip:     node.IP.String(),
		Port:   int64(node.Port),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert node address: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	res := convertNode(dbNode)
	nodeAddr := convertNodeAddress(dbNodeAddr)
	res.Addresses = append(res.Addresses, nodeAddr)
	return res, nil
}

func (r *NodesRepo) getDHTNodeAddressID(ctx context.Context, node *dht.Node) (int64, error) {
	return r.q.GetNodeAddress(ctx, &db.GetNodeAddressParams{
		PublicKey: (*db.PublicKey)(node.PublicKey),
		Net:       node.Type.Net(),
		Ip:        node.IP.String(),
		Port:      int64(node.Port),
	})
}

func (r *NodesRepo) PingDHTNode(ctx context.Context, node *dht.Node) error {
	id, err := r.getDHTNodeAddressID(ctx, node)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return r.q.PingNodeAddress(ctx, id)
}

func (r *NodesRepo) PongDHTNode(ctx context.Context, node *dht.Node) error {
	id, err := r.getDHTNodeAddressID(ctx, node)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return r.q.PongNodeAddress(ctx, id)
}

func (r *NodesRepo) GetResponsiveDHTNodes(ctx context.Context) ([]*dht.Node, error) {
	rows, err := r.q.GetResponsiveNodes(ctx)
	if err != nil {
		return nil, err
	}

	var combos []*nodeAddressCombo
	for _, row := range rows {
		combos = append(combos, &nodeAddressCombo{
			Node:        row.Node,
			NodeAddress: row.NodeAddress,
		})
	}

	return convertNodeAddressesToDHTNodes(combos)
}

func (r *NodesRepo) GetUnresponsiveDHTNodes(ctx context.Context, retryDelay time.Duration) ([]*dht.Node, error) {
	rows, err := r.q.GetUnresponsiveNodes(ctx, retryDelay.Seconds())
	if err != nil {
		return nil, err
	}

	var combos []*nodeAddressCombo
	for _, row := range rows {
		combos = append(combos, &nodeAddressCombo{
			Node:        row.Node,
			NodeAddress: row.NodeAddress,
		})
	}

	return convertNodeAddressesToDHTNodes(combos)
}

func convertNodeAddressesToDHTNodes(rows []*nodeAddressCombo) ([]*dht.Node, error) {
	// Only return a single address per node for now
	nodes := make(map[dht.PublicKey]*dht.Node)
	for _, row := range rows {
		publicKey := (*dht.PublicKey)(row.Node.PublicKey)
		if _, ok := nodes[*publicKey]; ok {
			continue
		}

		var nodeType dht.NodeType
		if err := nodeType.UnmarshalText([]byte(row.NodeAddress.Net)); err != nil {
			return nil, fmt.Errorf("convert db node: %w", err)
		}

		ip := net.ParseIP(row.NodeAddress.Ip)
		if ip == nil {
			return nil, fmt.Errorf("bad ip: %s", row.NodeAddress.Ip)
		}

		nodes[*publicKey] = &dht.Node{
			Type:      nodeType,
			PublicKey: publicKey,
			IP:        ip,
			Port:      int(row.NodeAddress.Port),
		}
	}

	return maps.Values(nodes), nil
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
