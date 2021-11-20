package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/songgao/water"
)

func setup(ips []string) *water.Interface {
	addr := strings.Split(ips[0], " ")[1]
	ip, _, err := net.ParseCIDR(addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	peer := strings.Split(ips[1], " ")[1]
	config := water.Config{
		DeviceType: water.TUN,
	}
	iface, err := water.New(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("interface => %v\n", iface.Name())
	runOutCmd("ifconfig %s inet %s %s netmask 255.255.255.255 up", iface.Name(), ip, peer)
	for _, val := range ips {
		vals := strings.Split(val, " ")
		fmt.Printf("control => %s\n", val)
		if vals[0] == "route" {
			if _, _, err := net.ParseCIDR(vals[1]); err != nil {
				fmt.Printf("unknown route => %v\n", vals[1])
			} else if strings.Contains(exclude, vals[1]) {
				fmt.Printf("exclude route => %v\n", vals[1])
			} else {
				rk := fmt.Sprintf("-net %s %s", vals[1], ip)
				clears[fmt.Sprintf("route -n delete %s", rk)] = true
				runCmd("route -n add %s", rk)
			}
		}
	}
	return iface
}
