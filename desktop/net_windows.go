package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/songgao/water"
)

func setup(iface *water.Interface, local, peer net.IP) {
	args := fmt.Sprintf("netsh interface ip set address \"%s\" static %s 255.255.255.255 %s", iface.Name(), local, peer)
	if out, err := runOutCmd(args); err != nil {
		logger.Warningf("%s\n", out)
		logger.Fatal(err)
	}
	ipStr := local.String()
	for {
		if out, err := runOutCmd(fmt.Sprintf("netsh interface ip show addresses \"%s\"", iface.Name())); err == nil {
			if strings.Contains(out, ipStr) && strings.Contains(out, "255.255.255.255") {
				break
			} else {
				fmt.Println("waiting network setup...")
				time.Sleep(time.Second)
			}
		} else {
			break
		}
	}
}

func addRoute(key string, peer net.IP) {
	ip, subnet, err := net.ParseCIDR(key)
	if err != nil {
		return
	}
	args := fmt.Sprintf("route add %s mask %s %s", ip, net.IP(subnet.Mask).String(), peer)
	if err := runCmd(args); err != nil {
		logger.Warning(err)
	}
}

func delRoute(key string) {
	ip, subnet, err := net.ParseCIDR(key)
	if err != nil {
		return
	}
	args := fmt.Sprintf("route delete %s mask %s %s", ip, net.IP(subnet.Mask).String(), peer)
	runCmd(args)
}
