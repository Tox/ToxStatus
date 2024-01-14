package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Tox/ToxStatus/internal/repo"
	"github.com/alexbakker/tox4go/dht"
	"github.com/alexbakker/tox4go/dht/ping"
	"github.com/alexbakker/tox4go/transport"
)

type Crawler struct {
	repo   *repo.NodesRepo
	opts   CrawlerOptions
	logger *slog.Logger

	m     sync.Mutex
	ident *dht.Identity
	pings *ping.Set

	started    atomic.Bool
	sendChan   chan *dhtPacket
	handleChan chan *dhtPacket
	recvChan   chan *rawPacket
}

type CrawlerOptions struct {
	Logger     *slog.Logger
	HTTPAddr   string
	ToxUDPAddr string
	Workers    int
}

type dhtPacket struct {
	Packet dht.Packet
	Node   *dht.Node
}

type rawPacket struct {
	Data []byte
	Addr *net.UDPAddr
}

func New(nodesRepo *repo.NodesRepo, opts CrawlerOptions) (*Crawler, error) {
	ident, err := dht.NewIdentity(dht.IdentityOptions{
		// Large cache for precomputed shared keys to improve performance
		SharedKeyCacheSize: 10000,
	})
	if err != nil {
		return nil, err
	}

	if opts.Workers < 2 || opts.Workers%2 != 0 {
		return nil, fmt.Errorf("bad number of workers: %d (must be a multiple of 2)", opts.Workers)
	}

	c := &Crawler{
		repo:       nodesRepo,
		opts:       opts,
		logger:     opts.Logger,
		ident:      ident,
		pings:      ping.NewSet(ping.DefaultTimeout),
		sendChan:   make(chan *dhtPacket),
		handleChan: make(chan *dhtPacket),
		recvChan:   make(chan *rawPacket),
	}

	return c, nil
}

func (c *Crawler) Run(ctx context.Context, bsNodes []*dht.Node) error {
	if !c.started.CompareAndSwap(false, true) {
		return errors.New("attempt to start crawler twice")
	}

	var err error
	var tp transport.Transport
	tp, err = transport.NewUDPTransport("udp", c.opts.ToxUDPAddr, func(data []byte, addr *net.UDPAddr) {
		// We need to copy the packet data, because once this function returns,
		// the backing buffer will be reused for the next packet, so the
		// contents of the data slice will get overwritten.
		cdata := make([]byte, len(data))
		copy(cdata, data)

		select {
		case <-ctx.Done():
			return
		case c.recvChan <- &rawPacket{Data: cdata, Addr: addr}:
		}
	})
	if err != nil {
		return fmt.Errorf("tox udp transport: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	listenErrChan := make(chan error)
	go func() {
		defer close(listenErrChan)

		if err := tp.Listen(); err != nil {
			listenErrChan <- err
		}
	}()

	workers := c.opts.Workers / 2
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			var total uint64
			logger := c.logger.With(slog.Int("worker", i))
			defer func() {
				logger.Info("Stopping packet transmitter", slog.Uint64("packets", total))
				wg.Done()
			}()

			logger.Info("Starting packet transmitter")

			for {
				select {
				case <-ctx.Done():
					return
				case packet := <-c.sendChan:
					if err := c.sendPacket(tp, packet.Packet, packet.Node); err != nil {
						c.logger.Error("Unable to send packet",
							slog.String("public_key", packet.Node.PublicKey.String()),
							slog.String("net", packet.Node.Type.Net()),
							slog.String("addr", packet.Node.Addr().String()),
							slog.Any("err", err))

						if errors.Is(err, net.ErrClosed) {
							return
						}
					}
				}

				total++
			}
		}(i)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			var total uint64
			logger := c.logger.With(slog.Int("worker", i))
			defer func() {
				logger.Info("Stopping packet receiver", slog.Uint64("packets", total))
				wg.Done()
			}()

			logger.Info("Starting packet receiver")

			for {
				select {
				case <-ctx.Done():
					return
				case packet := <-c.recvChan:
					if err := c.receivePacket(ctx, packet.Data, packet.Addr); err != nil {
						c.logger.Error("Unable to receive raw packet",
							slog.String("net", packet.Addr.Network()),
							slog.String("addr", packet.Addr.String()),
							slog.Any("err", err))

						if errors.Is(err, context.Canceled) {
							return
						}
					}
				}

				total++
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case packet := <-c.handleChan:
				if err := c.handleDHTPacket(ctx, tp, packet.Packet, packet.Node); err != nil {
					c.logger.Error("Unable to handle packet",
						slog.String("public_key", packet.Node.PublicKey.String()),
						slog.String("net", packet.Node.Type.Net()),
						slog.String("addr", packet.Node.Addr().String()),
						slog.Any("err", err))
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		c.logger.Info("Bootstrapping...", slog.Int("nodes", len(bsNodes)))

		for _, bsNode := range bsNodes {
			if err := ctx.Err(); err != nil {
				return
			}

			logger := slog.With(
				slog.String("public_key", bsNode.PublicKey.String()),
				slog.String("addr", bsNode.Addr().String()),
			)

			if _, err := c.repo.TrackDHTNode(ctx, bsNode); err != nil {
				logger.Error("Unable to track bootstrap node", slog.Any("err", err))
				continue
			}

			if err := c.getNodes(ctx, bsNode, c.ident.PublicKey); err != nil {
				logger.Error("Unable to query bootstrap node", slog.Any("err", err))
			}
		}

		// Wait for boostrapping to have gathered some node responses
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}

		// TODO: Remove nodes that we haven't successfully pinged in a while
		// Periodically query the nodes we know
		pkgen := getPublicKeyGenerator(199)
		var targetKeys []*dht.PublicKey
		for {
			c.logger.Info("Rotating target keys")
			for i := 0; i < 8; i++ {
				key := pkgen()
				targetKeys = append(targetKeys, key)
				c.logger.Info(key.String())
			}

			nodes, err := c.repo.GetResponsiveDHTNodes(ctx)
			if err == nil {
				c.logger.Info("Crawling...", slog.Int("nodes", len(nodes)))

				for _, node := range nodes {
					for _, targetKey := range targetKeys {
						if err := ctx.Err(); err != nil {
							return
						}
						if err := c.getNodes(ctx, node, targetKey); err != nil {
							c.logger.Error("Unable to query node",
								slog.String("public_key", node.PublicKey.String()),
								slog.String("addr", node.Addr().String()),
								slog.Any("err", err))
						}
					}
				}
			} else {
				c.logger.Error("Unable to obtain responsive dht nodes", slog.Any("err", err))
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			count, err := c.repo.GetNodeCount(ctx)
			if err == nil {
				c.logger.Info("Total number of nodes", slog.Int64("count", count))
			} else {
				c.logger.Error("Unable to query db for total number of nodes", slog.Any("err", err))
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			const retryPingDelay = 10 * time.Second
			nodes, err := c.repo.GetUnresponsiveDHTNodes(ctx, retryPingDelay)
			if err == nil {
				var pingedNodes int
				for _, node := range nodes {
					if err := ctx.Err(); err != nil {
						return
					}

					if err := c.getNodes(ctx, node, c.ident.PublicKey); err != nil {
						c.logger.Error("Unable to ping node",
							slog.String("public_key", node.PublicKey.String()),
							slog.String("addr", node.Addr().String()),
							slog.Any("err", err))
					} else {
						pingedNodes++
					}
				}

				c.logger.Info("Pinged nodes", slog.Int("count", pingedNodes))
			} else {
				c.logger.Error("Unable to obtain unresponsive dht nodes", slog.Any("err", err))
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()

	select {
	case err = <-listenErrChan:
		cancel()
	case <-ctx.Done():
		err = ctx.Err()
	}

	wg.Wait()
	tp.Close()
	<-listenErrChan
	return err
}

func (c *Crawler) handleDHTPacket(ctx context.Context, tp transport.Transport, packet dht.Packet, node *dht.Node) error {
	var err error
	switch packet := packet.(type) {
	case *dht.GetNodesPacket:
	case *dht.SendNodesPacket:
		err = c.handleSendNodesPacket(ctx, tp, node, packet)
	case *dht.PingRequestPacket:
	case *dht.PingResponsePacket:
	default:
		err = fmt.Errorf("unsupported packet type: %d", packet.ID())
	}

	return err
}

func (c *Crawler) handleSendNodesPacket(ctx context.Context, tp transport.Transport, node *dht.Node, packet *dht.SendNodesPacket) error {
	c.m.Lock()
	if _, err := c.pings.Pop(node.PublicKey, packet.PingID); err != nil {
		c.m.Unlock()
		return fmt.Errorf("unexpected sendnodes packet: %w", err)
	}
	c.m.Unlock()

	// Insert/update the known nodes list
	if err := c.repo.PongDHTNode(ctx, node); err != nil {
		return fmt.Errorf("update node pong time: %w", err)
	}

	var errs []error
	for _, packetNode := range packet.Nodes {
		// Don't query our own node or ones we've seen before
		if bytes.Equal(packetNode.PublicKey[:], c.ident.PublicKey[:]) {
			continue
		}
		found, err := c.repo.HasNodeByPublicKey(ctx, packetNode.PublicKey)
		if err != nil {
			return fmt.Errorf("check whether node is known: %w", err)
		}
		if found {
			continue
		}

		logger := c.logger.With(slog.String("public_key", packetNode.PublicKey.String()),
			slog.String("net", packetNode.Type.Net()),
			slog.String("addr", packetNode.Addr().String()))
		logger.Info("Tracking new node")

		if _, err := c.repo.TrackDHTNode(ctx, packetNode); err != nil {
			logger.Error("Unable to track node", slog.Any("err", err))
			continue
		}

		if err := c.getNodes(ctx, packetNode, c.ident.PublicKey); err != nil {
			errs = append(errs, err)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("query sent nodes: %w", errors.Join(errs...))
	}

	return nil
}

// getNodes queries the given DHT node to search for the given publicKey.
func (c *Crawler) getNodes(ctx context.Context, node *dht.Node, publicKey *dht.PublicKey) error {
	c.logger.Debug("Querying node",
		slog.String("public_key", node.PublicKey.String()),
		slog.String("net", node.Type.Net()),
		slog.String("addr", node.Addr().String()))

	c.m.Lock()
	ping, err := c.pings.Add(node.PublicKey)
	if err != nil {
		c.m.Unlock()
		return err
	}
	c.m.Unlock()

	if err := c.repo.PingDHTNode(ctx, node); err != nil {
		return fmt.Errorf("track node ping: %s", err)
	}

	packet := &dht.GetNodesPacket{
		PublicKey: publicKey,
		PingID:    ping.ID(),
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.sendChan <- &dhtPacket{Packet: packet, Node: node}:
	}

	return nil
}

func (c *Crawler) sendPacket(tp transport.Transport, packet dht.Packet, destNode *dht.Node) error {
	c.logger.Debug("Sending packet",
		slog.String("public_key", destNode.PublicKey.String()),
		slog.String("net", destNode.Type.Net()),
		slog.String("addr", destNode.Addr().String()),
		slog.String("packet_type", packet.ID().String()))

	dhtPacket, err := c.ident.EncryptPacket(packet, destNode.PublicKey)
	if err != nil {
		return err
	}

	packetBytes, err := dhtPacket.MarshalBinary()
	if err != nil {
		return err
	}

	return tp.SendPacket(packetBytes, &net.UDPAddr{IP: destNode.IP, Port: destNode.Port})
}

func (c *Crawler) receivePacket(ctx context.Context, data []byte, addr *net.UDPAddr) error {
	var nodeType dht.NodeType
	if addr.IP.To4() != nil {
		nodeType = dht.NodeTypeUDPIP4
	} else {
		nodeType = dht.NodeTypeUDPIP6
	}

	logger := c.logger.With(slog.String("net", nodeType.Net()), slog.String("addr", addr.String()))

	var encryptedPacket dht.EncryptedPacket
	if err := encryptedPacket.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("unmarshal encrypted packet: %w", err)
	}

	// We're only interested in sendnodes packets
	logger = logger.With(slog.String("packet_type", encryptedPacket.Type.String()))
	if encryptedPacket.Type != dht.PacketTypeSendNodes {
		logger.Debug("Ignoring non-sendnodes packet")
		return nil
	}
	logger.Debug("Handling packet")

	decryptedPacket, err := c.ident.DecryptPacket(&encryptedPacket)
	if err != nil {
		return fmt.Errorf("decrypt packet: %w", err)
	}

	node := &dht.Node{
		IP:        addr.IP,
		Port:      addr.Port,
		PublicKey: encryptedPacket.SenderPublicKey,
		Type:      nodeType,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.handleChan <- &dhtPacket{Packet: decryptedPacket, Node: node}:
	}

	return nil
}
