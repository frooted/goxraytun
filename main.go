package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/goxray/core/network/route"
	"github.com/goxray/tun/pkg/client"
)

var cmdArgsErr = `ERROR: no config_link provided
usage: %s [--tun-name name] [--tun-ip ip] [--no-routes] <config_url>
  - config_url - xray connection link, like "vless://example..."
  - or set GOXRAY_CONFIG_URL env var
`

func main() {
	tunIpStr := flag.String("tun-ip", "192.18.0.1", "TUN interface IP")
	tunName := flag.String("tun-name", "", "TUN interface name")
	noRoutes := flag.Bool("no-routes", false, "do not add routes")
	flag.Parse()

	// Get connection link from first cmd argument or env var.
	var clientLink string
	args := flag.Args()
	if len(args) > 1 {
		fmt.Printf(cmdArgsErr, os.Args[0])
		os.Exit(1)
	}
	if len(args) == 1 {
		clientLink = args[0]
	} else {
		clientLink = os.Getenv("GOXRAY_CONFIG_URL")
	}
	if clientLink == "" {
		fmt.Printf(cmdArgsErr, os.Args[0])
		os.Exit(1)
	}

	tunIp := net.ParseIP(*tunIpStr)
	if tunIp == nil {
		_, _ = fmt.Fprint(os.Stderr, "Invalid TUN IP format\n")
		os.Exit(1)
	}
	tunAddress := &net.IPNet{
		IP:   tunIp,
		Mask: net.CIDRMask(32, 32),
	}

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, os.Interrupt, syscall.SIGTERM)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	var routesToTUN []*route.Addr
	if *noRoutes {
		routesToTUN = []*route.Addr{}
	}

	vpn, err := client.NewClientWithOpts(client.Config{
		TLSAllowInsecure: false,
		Logger:           logger,
		TUNAddress:       tunAddress,
		TUNName:          *tunName,
		RoutesToTUN:      routesToTUN,
	})
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("Connecting to VPN server")
	err = vpn.Connect(clientLink)
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("Connected to VPN server")
	<-sigterm
	slog.Info("Received term signal, disconnecting...")
	if err = vpn.Disconnect(context.Background()); err != nil {
		slog.Warn("Disconnecting VPN failed", "error", err)
		os.Exit(0)
	}

	slog.Info("VPN disconnected successfully")
	os.Exit(0)
}
