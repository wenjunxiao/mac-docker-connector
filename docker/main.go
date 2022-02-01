package main

import (
	"bytes"
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
	MTU               = 1400
	debug             = false
	host              = "host.docker.internal"
	port              = 2511
	addr              = "192.168.251.1/24"
	heartbeat         = 5000
	iptablesInstalled = false
	chain             = "DOCKER-USER"
	dnsSvr            *DNSServer
)

func init() {
	flag.BoolVar(&debug, "debug", debug, "Provide debug info")
	flag.IntVar(&MTU, "mtu", MTU, "network MTU")
	flag.StringVar(&host, "host", host, "host to connect")
	flag.IntVar(&port, "port", port, "port to connect")
	flag.StringVar(&addr, "addr", addr, "virtual network address")
	flag.StringVar(&chain, "chain", chain, "iptables chain name")
	flag.IntVar(&heartbeat, "heartbeat", heartbeat, "heartbeat")
}

func runCmd(args string) string {
	argv := strings.Split(args, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out)
	}
	fmt.Printf("command error => %s %v\n", args, err)
	return ""
}

func getRoutes() map[string]string {
	routes := make(map[string]string)
	lines := strings.Split(runCmd("route -n"), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 2 {
			routes[fields[0]] = fields[len(fields)-1]
		}
	}
	return routes
}

func installIptables() {
	// iptables -L DOCKER-USER -vn --line-number
	if exec.Command("iptables", "-L", chain, "-vn", "--line-number").Run() != nil {
		fmt.Printf("iptables installing...\n")
		if err := exec.Command("apk", "add", "iptables").Run(); err != nil {
			fmt.Printf("iptables install failed => %v\n", err)
		} else {
			fmt.Printf("iptables installed\n")
		}
	}
}

func iptables(a, i, o string) error {
	// iptables -I DOCKER-USER -i br-net1 -o br-net2 -j ACCEPT
	// iptables -I DOCKER-USER -i br-net2 -o br-net1 -j ACCEPT
	if !iptablesInstalled {
		installIptables()
		iptablesInstalled = true
	}
	fmt.Printf("iptables %s %s -i %s -o %s\n", a, chain, i, o)
	cmd := exec.Command("iptables", a, chain, "-i", i, "-o", o, "-j", "ACCEPT")
	return cmd.Run()
}

// domain dot `.` in dns query is replaced by the length of next domain part
// such as `www.example.com` is converted to `www\x07example\x03com`
// use iptable hex-string, the content is `www|07|example|03|com|`
func hexDomain(s string) string {
	vals := strings.Split(s, ".")
	if len(vals) == 1 {
		return s
	}
	var buf bytes.Buffer
	if len(vals[0]) > 0 {
		buf.WriteString(vals[0])
	}
	buf.WriteString("|")
	for i := 1; i < len(vals); i++ {
		buf.WriteString(fmt.Sprintf("%02x", len(vals[i])))
		buf.WriteString("|")
		buf.WriteString(vals[i])
		buf.WriteString("|")
	}
	return buf.String()
}

// rediect the dns query of the domain to specified ip
// iptables -t nat -L PREROUTING -vn --line-number
func rediectDns(domains []string, ip net.IP) error {
	if !iptablesInstalled {
		installIptables()
		iptablesInstalled = true
	}
	for _, domain := range domains {
		act := "-I"
		if domain[0] == '-' {
			domain = domain[1:]
			act = "-D"
		} else {
			argv := []string{"iptables", "-t", "nat", "-D", "PREROUTING", "-p", "udp", "--dport", "53",
				"-m", "string", "--algo", "bm", "--hex-string", hexDomain(domain), "-j", "DNAT", "--to-destination", ip.String(),
			}
			fmt.Printf("clear dns => %v %v\n", strings.Join(argv, " "), exec.Command(argv[0], argv[1:]...).Run())
		}
		argv := []string{"iptables", "-t", "nat", act, "PREROUTING", "-p", "udp", "--dport", "53",
			"-m", "string", "--algo", "bm", "--hex-string", hexDomain(domain), "-j", "DNAT", "--to-destination", ip.String(),
		}
		fmt.Printf("dns command => %s %v\n", strings.Join(argv, " "), exec.Command(argv[0], argv[1:]...).Run())
	}
	return nil
}

func applyControls(cmds []string, ip net.IP) {
	routes := getRoutes()
	if dnsSvr != nil {
		dnsSvr.StartClear()
	}
	for _, val := range cmds {
		vals := strings.Split(val, " ")
		fmt.Printf("control => %s\n", val)
		switch vals[0] {
		case "connect":
			i1 := routes[vals[1]]
			i2 := routes[vals[2]]
			if len(i1) > 0 && len(i2) > 0 {
				if iptables("-C", i1, i2) != nil {
					iptables("-I", i1, i2)
					iptables("-I", i2, i1)
				}
			}
		case "disconnect":
			i1 := routes[vals[1]]
			i2 := routes[vals[2]]
			if len(i1) > 0 && len(i2) > 0 {
				iptables("-D", i1, i2)
				iptables("-D", i2, i1)
			}
		case "dns":
			rediectDns(vals[1:], ip)
		case "host":
			if dnsSvr == nil {
				dnsSvr = NewDnsServer()
			}
			dnsSvr.Add(strings.Join(vals[1:], " "))
		}
	}
	if dnsSvr != nil {
		dnsSvr.EndClear()
		dnsSvr.Start(ip)
	}
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
	ip, subnet, _ := net.ParseCIDR(addr)
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
	if err != nil {
		fmt.Printf("invalid command => %s\n", args)
		os.Exit(1)
	}
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
			if data[0] == 1 {
				var l int = 0
				l += int(data[1]) << 8
				l += int(data[2])
				var buf = make([]byte, l)
				var pos = n - 3
				copy(buf, data[3:n])
				for pos < l {
					if n, err = conn.Read(data); err != nil {
						fmt.Println("failed read udp msg, error: " + err.Error())
						break
					}
					copy(buf[pos:], data[:n])
					pos += n
				}
				if l > 0 {
					applyControls(strings.Split(string(buf), ","), ip)
				}
			} else {
				fmt.Printf("tun write error: %v\n", err)
			}
		}
		requested <- true
	}
}
