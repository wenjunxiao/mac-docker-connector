package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/songgao/water"
)

func setup(ips []string) *water.Interface {
	addr := strings.Split(ips[0], " ")[1]
	ip, subnet, err := net.ParseCIDR(addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	peer := strings.Split(ips[1], " ")[1]
	mask := net.IP(subnet.Mask).String()
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: "tap0901",
			Network:     addr,
		},
	}
	iface, err := water.New(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("interface => \"%s\"\n", iface.Name())
	runCmd(fmt.Sprintf("netsh interface ip set address \"%s\" static %s %s %s", iface.Name(), ip, mask, peer))
	ipStr := ip.String()
	for {
		if out, err := runCmd(fmt.Sprintf("netsh interface ip show addresses \"%s\"", iface.Name())); err == nil {
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
	for _, val := range ips {
		vals := strings.Split(val, " ")
		fmt.Printf("control => %s\n", val)
		if vals[0] == "route" {
			ip, subnet, err := net.ParseCIDR(vals[1])
			if err != nil {
				fmt.Printf("unknown route => %v\n", vals[1])
			} else if strings.Contains(exclude, vals[1]) {
				fmt.Printf("exclude route => %v\n", vals[1])
			} else {
				rk := fmt.Sprintf("%s mask %s %s", ip, net.IP(subnet.Mask).String(), peer)
				clears[fmt.Sprintf("route delete %s", rk)] = true
				runCmd(fmt.Sprintf("route add %s", rk))
			}
		}
	}
	return iface
}
