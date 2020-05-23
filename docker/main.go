package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/songgao/water"
)

var (
	// MTU maximum transmission unit
	MTU       = 1400
	debug     = false
	host      = "host.docker.internal"
	port      = 2511
	addr      = "192.168.251.1/24"
	heartbeat = 5000
)

func init() {
	flag.BoolVar(&debug, "debug", debug, "Provide debug info")
	flag.IntVar(&MTU, "mtu", MTU, "Provide debug info")
	flag.StringVar(&host, "host", host, "host listen")
	flag.IntVar(&port, "port", port, "port listen")
	flag.StringVar(&addr, "addr", addr, "address")
	flag.IntVar(&heartbeat, "heartbeat", heartbeat, "heartbeat")
}

func main() {
	flag.Parse()
	if _, err := os.Stat("/dev/net"); err != nil {
		os.Mkdir("/dev/net", os.ModePerm)
		cmd := exec.Command("mknod", "/dev/net/tun", "c", "10", "200")
		err = cmd.Run()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else if _, err := os.Stat("/dev/net/tun"); err != nil {
		cmd := exec.Command("mknod", "/dev/net/tun", "c", "10", "200")
		err = cmd.Run()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	config := water.Config{
		DeviceType: water.TUN,
	}
	iface, err := water.New(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("interface => %v\n", iface.Name())
	args := fmt.Sprintf("%s link set dev %s up mtu %d qlen 100", "ip", iface.Name(), MTU)
	argv := strings.Split(args, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	err = cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	ip, subnet, err := net.ParseCIDR(addr)
	peer := net.IP(make([]byte, 4))
	copy([]byte(peer), []byte(ip.To4()))
	peer[3]++
	args = fmt.Sprintf("%s addr add dev %s local %s peer %s", "ip", iface.Name(), ip, peer)
	fmt.Printf("command => %s\n", args)
	argv = strings.Split(args, " ")
	cmd = exec.Command(argv[0], argv[1:]...)
	err = cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	args = fmt.Sprintf("%s route add %s via %s dev %s", "ip", subnet, peer, iface.Name())
	fmt.Printf("command => %s\n", args)
	argv = strings.Split(args, " ")
	cmd = exec.Command(argv[0], argv[1:]...)
	err = cmd.Run()
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		fmt.Printf("invalid address => %s:%d\n", host, port)
		os.Exit(1)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		fmt.Printf("failed to dial %s:%d => %s\n", host, port, err.Error())
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("local => %s\n", conn.LocalAddr())
	fmt.Printf("remote => %s\n", conn.RemoteAddr())
	conn.Write([]byte{0})
	requested := make(chan bool, 1)
	go func() {
		buf := make([]byte, 2000)
		for {
			n, err := iface.Read(buf)
			if err != nil {
				fmt.Printf("tun read error: %v\n", err)
				continue
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				fmt.Printf("udp write error: %v\n", err)
			}
			requested <- true
		}
	}()
	go func() {
		duration := time.Duration(time.Millisecond * time.Duration(heartbeat))
		for {
			select {
			case <-requested:
				continue
			case <-time.After(duration):
				conn.Write([]byte{0})
			}
		}
	}()
	data := make([]byte, 2000)
	for {
		n, err := conn.Read(data)
		if err != nil {
			fmt.Println("failed read udp msg, error: " + err.Error())
		}
		if _, err := iface.Write(data[:n]); err != nil {
			fmt.Printf("tun write error: %v\n", err)
		}
		requested <- true
	}
}
