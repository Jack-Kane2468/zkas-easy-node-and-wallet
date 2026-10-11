package main

import (
	"strconv"
	"strings"
	"zkas-node-manager/internal/chains"
	"zkas-node-manager/internal/lanshare"
	"zkas-node-manager/internal/node"
)

func zLANRoutes(c node.Config) []lanshare.Route {
	routes := []lanshare.Route{{Port: c.GRPC}}
	if c.EnableWebSocket {
		routes = append(routes, lanshare.Route{Port: c.WebSocket})
	}
	if c.EnableWallet {
		routes = append(routes, lanshare.Route{Port: c.WalletPort, Handler: node.GatewayHandler(c.WalletPort, "", false)})
	}
	return routes
}
func kLANRoutes(c chains.Config) []lanshare.Route {
	return []lanshare.Route{{Port: c.GRPC}, {Port: c.JSON}, {Port: c.Borsh}}
}
func lanFirewallScript(name string, s node.SharingConfig, routes []lanshare.Route) string {
	ports := []string{}
	for _, r := range routes {
		ports = append(ports, strconv.Itoa(r.Port))
	}
	return "New-NetFirewallRule -Name " + psQuote(name) + " -DisplayName " + psQuote(name+" (private local network)") + " -Direction Inbound -Action Allow -Protocol TCP -LocalPort " + strings.Join(ports, ",") + " -LocalAddress " + psQuote(s.LANIP) + " -Profile Private,Domain -RemoteAddress LocalSubnet -Program " + psQuote(s.HostExecutable) + " | Out-Null;"
}
