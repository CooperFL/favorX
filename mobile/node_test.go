package mobile_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/FavorLabs/favorX/mobile"
)

func TestNewNode(t *testing.T) {
	var (
		node *mobile.Node
		opts *mobile.Options
		err  error
	)

	opts, err = mobile.ExportDefaultConfig()
	if err != nil {
		t.Fatal("export config:", err)
	}

	node, err = mobile.NewNode(opts)
	if err != nil {
		t.Fatal("new node:", err)
	}

	time.Sleep(3 * time.Second)

	err = node.Stop()
	if err != nil {
		t.Fatal("close node:", err)
	}
}

func TestHttpServeAndClose(t *testing.T) {
	opts, err := mobile.ExportDefaultConfig()
	if err != nil {
		t.Fatalf("export config: %v", err)
	}

	checkPort := func(t *testing.T, port int, mayClosed bool) {
		t.Helper()
		// check http port is listening?
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 1*time.Second)
		if mayClosed {
			if err == nil {
				t.Fatalf("port %d should be closed", port)
			}
		} else {
			if err != nil {
				t.Fatalf("dial port %d: %v", port, err)
			}
			if conn != nil {
				_ = conn.Close()
			}
		}
	}

	opts.EnableTLS = false
	opts.EnableDebugAPI = true

	node, err := mobile.NewNode(opts)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	time.Sleep(3 * time.Second)
	checkPort(t, opts.ApiPort, false)
	checkPort(t, opts.DebugAPIPort, false)
	checkPort(t, opts.WebsocketPort, false)

	err = node.StopNetwork(3)
	if err != nil {
		t.Fatalf("shutting down node listening port: %v", err)
	}

	time.Sleep(3 * time.Second)
	checkPort(t, opts.ApiPort, true)
	checkPort(t, opts.DebugAPIPort, true)
	checkPort(t, opts.WebsocketPort, true)

	port, err := node.StartNetwork()
	if err != nil {
		t.Fatalf("recovering node listening port: %v", err)
	}

	if port == 0 {
		t.Fatalf("node http not started")
	}

	t.Logf("node listen on port %d", port)

	time.Sleep(3 * time.Second)
	checkPort(t, opts.ApiPort, false)
	checkPort(t, opts.DebugAPIPort, false)
	checkPort(t, opts.WebsocketPort, false)

	err = node.Stop()
	if err != nil {
		t.Fatal("close node:", err)
	}
}
