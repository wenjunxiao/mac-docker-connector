package main

import (
	"fmt"
	"net"

	"github.com/songgao/water"
)

func setup(iface *water.Interface, local, peer net.IP) {
	args := fmt.Sprintf("%s %s inet %s %s netmask 255.255.255.255 up", "ifconfig", iface.Name(), local, peer)
	if err := runCmd(args); err != nil {
		logger.Fatal(err)
	}
	args = fmt.Sprintf("route -n add -host %s -interface %s", local, iface.Name())
	if err := runCmd(args); err != nil {
		logger.Warning(err)
	}
	logger.Info("drawin setup done.")
}

func addRoute(key string, peer net.IP) {
	args := fmt.Sprintf("route -n add -net %s %s", key, peer)
	if err := runCmd(args); err != nil {
		logger.Warning(err)
	}
}

func delRoute(key string) {
	runCmd(fmt.Sprintf("route -n delete -net %s", key))
}
