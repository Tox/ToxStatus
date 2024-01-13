package repo

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
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
