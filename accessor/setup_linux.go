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
	runOutCmd(fmt.Sprintf("%s link set dev %s up qlen 100", "ip", iface.Name()))
	runOutCmd(fmt.Sprintf("%s addr add dev %s local %s peer %s", "ip", iface.Name(), ip, peer))
	for _, val := range ips {
		vals := strings.Split(val, " ")
		fmt.Printf("control => %s\n", val)
		if vals[0] == "route" {
			if _, _, err := net.ParseCIDR(vals[1]); err != nil {
				fmt.Printf("unknown route => %v\n", vals[1])
			} else if strings.Contains(exclude, vals[1]) {
				fmt.Printf("exclude route => %v\n", vals[1])
			} else {
				rk := fmt.Sprintf("%s via %s dev %s", vals[1], ip, iface.Name())
				clears[fmt.Sprintf("ip route del %s", rk)] = true
				runCmd(fmt.Sprintf("ip route add %s", rk))
			}
		} else if vals[0] == "mtu" {
			runOutCmd(fmt.Sprintf("%s link set dev %s mtu %s", "ip", iface.Name(), vals[1]))
		}
	}

	return iface
}
