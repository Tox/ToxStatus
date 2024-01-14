package repo

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"net"
	"testing"

	"github.com/Tox/ToxStatus/internal/db"
	"github.com/Tox/ToxStatus/internal/models"
	"github.com/alexbakker/tox4go/dht"
	_ "github.com/mattn/go-sqlite3"
)

var ctx = context.Background()

func initRepo(t *testing.T) (repo *NodesRepo, close func() error) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := dbConn.ExecContext(ctx, db.Schema); err != nil {
		t.Fatal(err)
	}

	return New(dbConn), dbConn.Close
}

func generateNode(t *testing.T) *models.Node {
	return &models.Node{
		PublicKey: generatePublicKey(t),
	}
}

func generateIP(t *testing.T) net.IP {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		t.Fatal(err)
	}

	return net.IP(bytes)
}

func generateDHTNode(t *testing.T) *dht.Node {
	return &dht.Node{
		Type:      dht.NodeTypeUDPIP4,
		PublicKey: generatePublicKey(t),
		IP:        generateIP(t),
		Port:      33445,
	}
}

func generatePublicKey(t *testing.T) *dht.PublicKey {
	ident, err := dht.NewIdentity(dht.IdentityOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return ident.PublicKey
}

func TestAddNode(t *testing.T) {
	repo, close := initRepo(t)
	defer close()

	node := generateNode(t)
	dbNode, err := repo.q.UpsertNode(ctx, &db.UpsertNodeParams{PublicKey: (*db.PublicKey)(node.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(node.PublicKey[:], dbNode.PublicKey[:]) {
		t.Fatal("public keys not equal")
	}
}

func TestGetNonExistentNode(t *testing.T) {
	repo, close := initRepo(t)
	defer close()

	_, err := repo.GetNodeByPublicKey(ctx, generatePublicKey(t))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected error: '%v', got: %v", ErrNotFound, err)
	}
}

func TestHasNodeByPublicKey(t *testing.T) {
	repo, close := initRepo(t)
	defer close()

	node := generateNode(t)
	_, err := repo.q.UpsertNode(ctx, &db.UpsertNodeParams{PublicKey: (*db.PublicKey)(node.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}

	found, err := repo.HasNodeByPublicKey(ctx, node.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("unable to find node by public key")
	}

	found, err = repo.HasNodeByPublicKey(ctx, generatePublicKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("found non-existent node by public key")
	}
}

func TestPongNonExistentNode(t *testing.T) {
	repo, close := initRepo(t)
	defer close()

	pk := generatePublicKey(t)
	_, err := repo.q.UpsertNode(ctx, &db.UpsertNodeParams{PublicKey: (*db.PublicKey)(pk)})
	if err != nil {
		t.Fatal(err)
	}

	node := generateDHTNode(t)
	if err := repo.PongDHTNode(ctx, node); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected error: '%v', got: %v", ErrNotFound, err)
	}
}
