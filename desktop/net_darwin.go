package main

import (
	"net"

	"github.com/songgao/water"
)

func setup(local, peer net.IP, subnet *net.IPNet) *water.Interface {
	config := water.Config{
		DeviceType: water.TUN,
	}
	iface, err := water.New(config)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("interface => %s\n", iface.Name())
	if out, err := runOutCmd("%s %s inet %s %s netmask 255.255.255.255 up", "ifconfig", iface.Name(), local, peer); err != nil {
		logger.Warningf("%s\n", out)
		logger.Fatal(err)
	}
	if err := runCmd("route -n add -host %s -interface %s", local, iface.Name()); err != nil {
		logger.Warning(err)
	}
	logger.Info("drawin setup done.")
	return iface
}

func addRoute(key string, peer net.IP) {
	if err := runCmd("route -n add -net %s %s", key, peer); err != nil {
		logger.Warning(err)
	}
}

func delRoute(key string) {
	runCmd("route -n delete -net %s", key)
}
