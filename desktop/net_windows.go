package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/songgao/water"
)

func setup(local, peer net.IP, subnet *net.IPNet) *water.Interface {
	ones, _ := subnet.Mask.Size()
	mask := net.IP(subnet.Mask).String()
	addr := fmt.Sprintf("%s/%d", local, ones)
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: "tap0901",
			Network:     addr,
		},
	}
	iface, err := water.New(config)
	if err != nil {
		logger.Fatal(err)
	}
	if out, err := runOutCmd("netsh interface ip set address \"%s\" static %s %s %s", iface.Name(), local, mask, peer); err != nil {
		logger.Warningf("%s\n", out)
		logger.Fatal(err)
	}
	ipStr := local.String()
	for {
		if out, err := runOutCmd("netsh interface ip show addresses \"%s\"", iface.Name()); err == nil {
			if strings.Contains(out, ipStr) && strings.Contains(out, mask) {
				break
			} else {
				fmt.Println("waiting network setup...")
				time.Sleep(time.Second)
			}
		} else {
			break
		}
	}
	runCmd("netsh interface ip delete dns \"%s\" all", iface.Name())
	runCmd("netsh interface ip delete wins \"%s\" all", iface.Name())
	return iface
}

func addRoute(key string, peer net.IP) {
	ip, subnet, err := net.ParseCIDR(key)
	if err != nil {
		return
	}
	if err := runCmd("route add %s mask %s %s", ip, net.IP(subnet.Mask).String(), peer); err != nil {
		logger.Warning(err)
	}
}

func delRoute(key string) {
	ip, subnet, err := net.ParseCIDR(key)
	if err != nil {
		return
	}
	runCmd("route delete %s mask %s %s", ip, net.IP(subnet.Mask).String(), peer)
}
