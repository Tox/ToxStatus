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
		addr := convertNodeAddress(node, &row.NodeAddress)
		node.Addresses = append(node.Addresses, addr)
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
	nodeAddr := convertNodeAddress(res, dbNodeAddr)
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

func (r *NodesRepo) GetNodesWithStaleBootstrapInfo(ctx context.Context) ([]*models.Node, error) {
	rows, err := r.q.GetNodesWithStaleBootstrapInfo(ctx, &db.GetNodesWithStaleBootstrapInfoParams{
		NodeTimeout:  (5 * time.Minute).Seconds(),
		InfoInterval: (1 * time.Minute).Seconds(),
	})
	if err != nil {
		return nil, err
	}

	nodes := make(map[dht.PublicKey]*models.Node)
	for _, row := range rows {
		node, ok := nodes[dht.PublicKey(*row.Node.PublicKey)]
		if !ok {
			node = convertNode(&row.Node)
			nodes[*node.PublicKey] = node
		}

		addr := convertNodeAddress(node, &row.NodeAddress)
		node.Addresses = append(node.Addresses, addr)
	}

	return maps.Values(nodes), nil
}

func (r *NodesRepo) UpdateNodeInfoRequestTime(ctx context.Context, addrReqTimes map[int64]time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := r.q.WithTx(tx)
	for id, reqTime := range addrReqTimes {
		if err := q.UpdateNodeInfoRequestTime(ctx, &db.UpdateNodeInfoRequestTimeParams{
			ID:            id,
			LastInfoReqAt: db.Time(reqTime),
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *NodesRepo) UpdateNodeInfo(ctx context.Context, addr *net.UDPAddr, motd string, version uint32) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var nodeType dht.NodeType
	if addr.IP.To4() != nil {
		nodeType = dht.NodeTypeUDPIP4
	} else {
		nodeType = dht.NodeTypeUDPIP6
	}

	q := r.q.WithTx(tx)
	node, err := r.q.GetNodeByInfoResponseAddress(ctx, &db.GetNodeByInfoResponseAddressParams{
		InfoReqTimeout: (10 * time.Second).Seconds(),
		Net:            nodeType.Net(),
		Ip:             addr.IP.String(),
		Port:           int64(addr.Port),
	})
	if err != nil {
		return err
	}

	if err := q.UpdateNodeBootstrapInfo(ctx, &db.UpdateNodeBootstrapInfoParams{
		PublicKey: node.Node.PublicKey,
		Motd:      newNullString(&motd),
		Version:   sql.NullInt64{Valid: true, Int64: int64(version)},
	}); err != nil {
		return err
	}

	return tx.Commit()
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
		// TODO: Replace with models.NodeAddress.DHTNode()
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
		ID:            dbNode.ID,
		CreatedAt:     time.Time(dbNode.CreatedAt),
		LastSeenAt:    time.Time(dbNode.LastSeenAt),
		LastInfoReqAt: time.Time(dbNode.LastInfoReqAt),
		LastInfoResAt: time.Time(dbNode.LastInfoResAt),
		PublicKey:     (*dht.PublicKey)(dbNode.PublicKey),
		FQDN:          convertNullString(dbNode.Fqdn),
		MOTD:          convertNullString(dbNode.Motd),
		Version:       uint32(dbNode.Version.Int64),
	}
}

func convertNodeAddress(node *models.Node, dbNodeAddr *db.NodeAddress) *models.NodeAddress {
	return &models.NodeAddress{
		Node:       node,
		ID:         dbNodeAddr.ID,
		CreatedAt:  time.Time(dbNodeAddr.CreatedAt),
		LastSeenAt: time.Time(dbNodeAddr.LastSeenAt),
		LastPingAt: time.Time(dbNodeAddr.LastPingAt),
		LastPongAt: time.Time(dbNodeAddr.LastPongAt),
		Net:        dbNodeAddr.Net,
		IP:         dbNodeAddr.Ip,
		Port:       int(dbNodeAddr.Port),
		Ptr:        convertNullString(dbNodeAddr.Ptr),
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
