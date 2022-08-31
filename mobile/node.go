package mobile

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	fx "github.com/FavorLabs/favorX"
	"github.com/FavorLabs/favorX/pkg/keystore/p2pkey"
	"github.com/FavorLabs/favorX/pkg/node"
	"github.com/gauss-project/aurorafs/pkg/aurora"
	"github.com/gauss-project/aurorafs/pkg/boson"
	"github.com/gauss-project/aurorafs/pkg/crypto"
	filekeystore "github.com/gauss-project/aurorafs/pkg/keystore/file"
	"github.com/gauss-project/aurorafs/pkg/logging"
	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/sirupsen/logrus"
)

type Node struct {
	node       *node.Favor
	opts       node.Options
	logger     logging.Logger
	netMux     sync.Mutex
	netEnabled bool
}

type signerConfig struct {
	signer           crypto.Signer
	address          boson.Address
	publicKey        *ecdsa.PublicKey
	libp2pPrivateKey crypto2.PrivKey
}

func Version() string {
	return fx.Version
}

func NewNode(o *Options) (*Node, error) {
	logger, err := newLogger(o.Verbosity)
	if err != nil {
		return nil, err
	}

	// put keys into dataDir
	keyPath := filepath.Join(o.DataPath, "keys")

	signerConfig, err := configureSigner(keyPath, o.Password, uint64(o.NetworkID), logger)
	if err != nil {
		return nil, err
	}

	logger.Infof("version: %v", Version())

	mode := aurora.NewModel()
	if o.EnableFullNode {
		mode.SetMode(aurora.FullNode)
		logger.Info("start node mode full.")
	} else {
		logger.Info("start node mode light.")
	}

	config := o.export()
	p2pAddr := fmt.Sprintf("%s:%d", listenAddress, o.P2PPort)

	n, err := node.NewNode(mode, p2pAddr, signerConfig.address, *signerConfig.publicKey, signerConfig.signer, uint64(o.NetworkID), logger, signerConfig.libp2pPrivateKey, config)
	if err != nil {
		return nil, err
	}

	return &Node{node: n, netEnabled: true, opts: config, logger: logger}, nil
}

var (
	ErrNetworkReady  = errors.New("network ready")
	ErrNetworkClosed = errors.New("network closed")
)

func (n *Node) StartNetwork() (int, error) {
	n.netMux.Lock()
	defer n.netMux.Unlock()

	if n.netEnabled {
		apiPort := n.node.ListenOn("api")
		return int(apiPort), ErrNetworkReady
	}

	n.netEnabled = true

	err := n.node.HttpServe(
		n.opts.APIAddr,
		n.opts.DebugAPIAddr,
		n.opts.EnableApiTLS,
		n.opts.TlsCrtFile,
		n.opts.TlsKeyFile,
		n.logger,
	)
	if err != nil {
		return 0, err
	}

	apiPort := n.node.ListenOn("api")
	return int(apiPort), nil
}

func (n *Node) StopNetwork(wait int) error {
	n.netMux.Lock()
	defer n.netMux.Unlock()

	if !n.netEnabled {
		return ErrNetworkClosed
	}

	n.netEnabled = false

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wait)*time.Second)
	defer cancel()

	return n.node.HttpShutdown(ctx)
}

func (n *Node) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return n.node.Shutdown(ctx)
}

func configureSigner(path, password string, networkID uint64, logger logging.Logger) (*signerConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("keystore directory not provided")
	}

	keystore := filekeystore.New(path)

	PrivateKey, created, err := keystore.Key("boson", password)
	if err != nil {
		return nil, fmt.Errorf("boson key: %w", err)
	}
	signer := crypto.NewDefaultSigner(PrivateKey)
	publicKey := &PrivateKey.PublicKey

	address, err := crypto.NewOverlayAddress(*publicKey, networkID)
	if err != nil {
		return nil, err
	}
	if created {
		logger.Infof("new boson network address created: %s", address)
	} else {
		logger.Infof("using existing boson network address: %s", address)
	}

	logger.Infof("boson public key %x", crypto.EncodeSecp256k1PublicKey(publicKey))

	libp2pPrivateKey, created, err := p2pkey.New(path).Key("libp2p", password)
	if err != nil {
		return nil, fmt.Errorf("libp2p key: %w", err)
	}
	if created {
		logger.Debugf("new libp2p key created")
	} else {
		logger.Debugf("using existing libp2p key")
	}

	return &signerConfig{
		signer:           signer,
		address:          address,
		publicKey:        publicKey,
		libp2pPrivateKey: libp2pPrivateKey,
	}, nil
}

func cmdOutput() io.Writer {
	return os.Stdout
}

func newLogger(verbosity string) (logging.Logger, error) {
	var logger logging.Logger
	switch verbosity {
	case "0", "silent":
		logger = logging.New(io.Discard, 0)
	case "1", "error":
		logger = logging.New(cmdOutput(), logrus.ErrorLevel)
	case "2", "warn":
		logger = logging.New(cmdOutput(), logrus.WarnLevel)
	case "3", "info":
		logger = logging.New(cmdOutput(), logrus.InfoLevel)
	case "4", "debug":
		logger = logging.New(cmdOutput(), logrus.DebugLevel)
	case "5", "trace":
		logger = logging.New(cmdOutput(), logrus.TraceLevel)
	default:
		return nil, fmt.Errorf("unknown verbosity level %q", verbosity)
	}

	return logger, nil
}
