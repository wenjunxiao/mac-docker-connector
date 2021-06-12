package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/op/go-logging"
	"github.com/songgao/water"
)

// TmpPeer peer tmp file
const TmpPeer = "/tmp/mac-docker-connector.peer"

var (
	logger *logging.Logger
	conn   *net.UDPConn
	cli    *net.UDPAddr
	peer   net.IP
	expose *net.UDPConn
	subnet *net.IPNet
	// MTU maximum transmission unit
	MTU        = 1400
	host       = "127.0.0.1"
	port       = 2511
	addr       = "192.168.251.1/24"
	configFile = ""
	watch      = true
	routes     = make(map[string]bool)
	tokens     = make(map[string]string)
	iptables   = make(map[string]bool)
	logLevel   = "INFO"
	sessions   = make(map[uint64]*net.UDPAddr)
	localIP    = net.IP(make([]byte, 4))
	pong       = false
	cliAddr    = ""
)

func init() {
	logging.SetLevel(logging.INFO, "vpn")
	logger = logging.MustGetLogger("vpn")
	flag.IntVar(&MTU, "mtu", MTU, "mtu")
	flag.StringVar(&host, "host", host, "udp listen host")
	flag.IntVar(&port, "port", port, "udp listen port")
	flag.StringVar(&addr, "addr", addr, "virtual network address")
	flag.StringVar(&configFile, "config", configFile, "config file")
	flag.BoolVar(&watch, "watch", watch, "watch config file")
	flag.StringVar(&logLevel, "log-level", logLevel, "log level")
	flag.BoolVar(&pong, "pong", pong, "pong")
	flag.StringVar(&cliAddr, "cli", cliAddr, "udp client address")
}

func runCmd(args string) error {
	argv := strings.Split(args, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	logger.Infof("command => %s", args)
	return cmd.Run()
}

func checkSum(data []byte) uint16 {
	var (
		sum    uint32
		length = len(data)
		index  int
	)
	//以每16位为单位进行求和，直到所有的字节全部求完或者只剩下一个8位字节（如果剩余一个8位字节说明字节数为奇数个）
	for length > 1 {
		sum += uint32(data[index]) + uint32(data[index+1])<<8
		index += 2
		length -= 2
	}
	//如果字节数为奇数个，要加上最后剩下的那个8位字节
	if length > 0 {
		sum += uint32(data[index])
	}
	//加上高16位进位的部分
	sum += (sum >> 16)
	//别忘了返回的时候先求反
	return uint16(^sum)
}

func handleExpose() {
	defer expose.Close()
	data := make([]byte, 2000)
	users := make(map[string]bool)
	for {
		n, addr, err := expose.ReadFromUDP(data)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				logger.Info("expose server closed.")
				return
			}
			logger.Warningf("failed read udp msg, error: %v\n", err)
			continue
		}
		if users[addr.String()] {
			if pong {
				if data[0]&0xf0 == 0x40 { // IPv4
					total := 256*uint64(data[2]) + uint64(data[3]) // 总长度
					packet := data[:total]
					if packet[9] == 0x01 { // ICMPv4
						if packet[20] == 0x08 { // IPv4 echo request
							var echoReply bytes.Buffer
							echoReply.Write(packet[:12])
							echoReply.Write(packet[16:20])
							echoReply.Write(packet[12:16])
							echoReply.WriteByte(0x00)
							echoReply.Write(packet[21:])
							reply := echoReply.Bytes()
							reply[22] = 0x00
							reply[23] = 0x00
							crc := checkSum(reply[20:])
							reply[22] = byte((crc & 0x00ff) >> 0)
							reply[23] = byte((crc & 0xff00) >> 8)
							logger.Debugf("Send IPv4 echo reply => %v %v\n", addr.String(), packetIP(packet))
							expose.WriteToUDP(reply, addr)
							continue
						} else if packet[20] == 0x00 {
							logger.Debugf("Received IPv4 echo reply => %v %v\n", addr.String(), packetIP(packet))
						}
					}
				} else if data[0]&0xf0 == 0x60 { // IPv6
					// not supported
				}
			}
			if _, err := conn.WriteToUDP(data[:n], cli); err != nil {
				logger.Warningf("udp write error: %v\n", err)
			}
		} else if data[0] == 1 {
			token := string(data[1:n])
			clientIP := addr.String()
			logger.Debugf("client token => %s %s\n", clientIP, token)
			if ip, ok := tokens[token]; ok {
				users[clientIP] = true
				intIP := toIntIP(net.ParseIP(ip).To4(), 0, 1, 2, 3)
				logger.Infof("client session => %s %s %v\n", clientIP, ip, intIP)
				sessions[intIP] = addr
				var reply bytes.Buffer
				reply.WriteByte(1)
				// 验证成功返回IP
				ones, _ := subnet.Mask.Size()
				reply.WriteString(fmt.Sprintf("addr %s/%d", ip, ones))
				reply.WriteString(fmt.Sprintf(",peer %s", localIP.String()))
				reply.WriteString(fmt.Sprintf(",mtu %d", MTU))
				for k, v := range routes {
					if v {
						reply.WriteString(",route ")
						reply.WriteString(k)
					}
				}
				logger.Infof("reply client => %s %d %s %s\n", clientIP, reply.Len(), reply.String(), addr)
				expose.WriteToUDP(reply.Bytes(), addr)
			} else {
				logger.Infof("invalid token => %s %s\n", clientIP, token)
			}
		} else {
			expose.WriteToUDP([]byte{2}, addr)
		}
	}
}

func normalizeAddr(addr string) string {
	if strings.Index(addr, "[::]") == 0 {
		return strings.Replace(addr, "[::]", "0.0.0.0", 1)
	}
	return addr
}

func isSameAddr(addr0, addr1 string) bool {
	if addr0 == addr1 {
		return true
	}
	return normalizeAddr(addr0) == normalizeAddr(addr1)
}

func loadConfig(iface *water.Interface, init bool) {
	fi, err := os.Open(configFile)
	if err != nil {
		return
	}
	defer fi.Close()
	re := regexp.MustCompile(`^\s*(\w+\S+)(?:\s+(.*))?$`)
	news := make(map[string]bool)
	news1 := make(map[string]string)
	iptables1 := make(map[string]bool)
	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		s := string(a)
		match := re.FindStringSubmatch(s)
		if match != nil {
			val := match[2]
			switch match[1] {
			case "route":
				vals := strings.Split(val, " ")
				if len(vals) > 1 {
					news[vals[0]] = vals[1] == "expose"
				} else {
					news[vals[0]] = false
				}
			case "host":
				host = val
			case "addr":
				addr = val
			case "port":
				if v, err := strconv.Atoi(val); err == nil {
					port = v
				}
			case "mtu":
				if v, err := strconv.Atoi(val); err == nil {
					MTU = v
				}
			case "pong":
				pong = val == "on" || val == "true"
			case "expose":
				restart := strings.Contains(val, "restart")
				val = strings.Fields(val)[0]
				if udpAddr, err := net.ResolveUDPAddr("udp", val); err == nil {
					if expose != nil && (restart || !isSameAddr(expose.LocalAddr().String(), val)) {
						logger.Infof("expose changed: %s => %s\n", expose.LocalAddr().String(), val)
						expose.Close()
						expose = nil
					}
					if expose == nil {
						if expose, err = net.ListenUDP("udp", udpAddr); err != nil {
							logger.Warningf("failed to listen => %s\n", val)
						} else {
							go handleExpose()
						}
					}
				} else {
					logger.Warningf("invalid address => %s\n", val)
				}
			case "token":
				vals := strings.Split(val, " ")
				news1[vals[0]] = vals[1]
			case "iptables":
				vals := strings.Split(val, "+")
				join := true
				if len(vals) == 1 {
					vals = strings.Split(val, "-")
					join = false
				}
				val = fmt.Sprintf("%s %s", vals[0], vals[1])
				if vals[0] > vals[1] {
					val = fmt.Sprintf("%s %s", vals[1], vals[0])
				}
				iptables1[val] = join
			}
		}
	}
	if init {
		if peer, subnet, err = net.ParseCIDR(addr); err != nil {
			logger.Fatal(err)
		}
		copy([]byte(localIP), []byte(peer.To4()))
		localIP[3]++
		args := fmt.Sprintf("%s %s inet %s %s netmask 255.255.255.255 up", "ifconfig", iface.Name(), localIP, peer)
		if err = runCmd(args); err != nil {
			logger.Fatal(err)
		}
		for k, v := range iptables1 {
			iptables[k] = v
		}
	}
	for key := range routes {
		if val, ok := news[key]; ok {
			routes[key] = val
			delete(news, key)
		} else {
			args := fmt.Sprintf("route -n delete -net %s", key)
			runCmd(args)
		}
	}
	for key := range news {
		routes[key] = news[key]
		runCmd(fmt.Sprintf("route -n delete -net %s", key))
		args := fmt.Sprintf("route -n add -net %s %s", key, peer)
		if err = runCmd(args); err != nil {
			logger.Warning(err)
		}
	}
	for key := range tokens {
		if v, ok := news1[key]; ok {
			tokens[key] = v
		} else {
			delete(tokens, key)
		}
	}
	for key := range news1 {
		tokens[key] = news1[key]
	}
	for key := range iptables {
		if v, ok := iptables1[key]; ok {
			if iptables[key] == v {
				delete(iptables1, key)
			} else {
				iptables[key] = v
			}
		} else {
			delete(iptables, key)
			iptables1[key] = false
		}
	}
	if cli != nil && len(iptables1) > 0 {
		sendIptable(cli, iptables1)
	}
	news = nil
	news1 = nil
	iptables1 = nil
}

func packetIP(data []byte) net.IP {
	return net.IPv4(data[16], data[17], data[18], data[19])
}

func sendIptable(cli *net.UDPAddr, tables map[string]bool) {
	logger.Infof("send iptables => %v %v\n", cli, tables)
	var reply bytes.Buffer
	for k, v := range tables {
		if reply.Len() > 0 {
			reply.WriteString(",")
		}
		if v {
			reply.WriteString("connect ")
		} else {
			reply.WriteString("disconnect ")
		}
		reply.WriteString(k)
	}
	l := reply.Len()
	if l > 0 {
		if l < 50 {
			logger.Infof("reply client => %s %d %s\n", cli, l, reply.String())
		} else {
			logger.Infof("reply client => %s %d\n", cli, l)
		}
		l16 := uint16(l)
		header := make([]byte, 3)
		header[0] = 1
		header[1] = byte(l16 >> 8)
		header[2] = byte(l16 & 0x00ff)
		if _, err := conn.WriteToUDP(header, cli); err != nil {
			logger.Warningf("write header error: %v %v\n", header, err)
		}
		tmp := reply.Bytes()
		for i := 0; i < l; i += MTU {
			if _, err := conn.WriteToUDP(tmp[i:min(i+MTU, l)], cli); err != nil {
				logger.Warningf("write body error: %v %v\n", l, err)
			}
		}
	}
}

func main() {
	flag.Parse()
	if level, err := logging.LogLevel(logLevel); err == nil {
		logging.SetLevel(level, "vpn")
	}
	config := water.Config{
		DeviceType: water.TUN,
	}
	iface, err := water.New(config)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("interface => %s\n", iface.Name())
	if _, err := os.Stat(configFile); err == nil {
		logger.Infof("load config(%v) => %s\n", watch, configFile)
		loadConfig(iface, true)
		if watch {
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				logger.Fatal(err)
			}
			defer watcher.Close()
			go func() {
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						if event.Op&fsnotify.Write == fsnotify.Write {
							logger.Debugf("config file changed => %s\n", configFile)
							loadConfig(iface, false)
						} else if event.Op&fsnotify.Rename == fsnotify.Rename {
							logger.Debugf("config file renamed => %s\n", event.Name)
							loadConfig(iface, false)
							if err = watcher.Remove(configFile); err != nil {
								logger.Warningf("remove watch error => %v\n", err)
							}
							if err = watcher.Add(event.Name); err != nil {
								logger.Warningf("watch error => %v\n", err)
							}
						}
					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						logger.Info("error:", err)
					}
				}
			}()
			if err = watcher.Add(configFile); err == nil {
				if full, err := filepath.Abs(configFile); err != nil {
					logger.Debugf("watch config => %s\n", full)
				} else {
					logger.Debugf("watch config => %s\n", configFile)
				}
			} else {
				logger.Warningf("watch error => %v\n", err)
			}
		}
	} else {
		if peer, subnet, err = net.ParseCIDR(addr); err != nil {
			logger.Fatal(err)
		}
		copy([]byte(localIP), []byte(peer.To4()))
		localIP[3]++
		args := fmt.Sprintf("%s %s inet %s %s netmask 255.255.255.255 up", "ifconfig", iface.Name(), localIP, peer)
		if err = runCmd(args); err != nil {
			logger.Fatal(err)
		}
	}
	args := fmt.Sprintf("route -n add -host %s -interface %s", localIP, iface.Name())
	if err = runCmd(args); err != nil {
		logger.Warning(err)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		logger.Fatalf("invalid address => %s:%d", host, port)
	}
	// 监听
	conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		logger.Fatalf("failed to listen %s:%d => %s", host, port, err.Error())
		return
	}
	defer conn.Close()
	logger.Infof("listen => %v\n", conn.LocalAddr())
	if cliAddr == "" {
		if tmp, err := ioutil.ReadFile(TmpPeer); err == nil {
			if cli, err = net.ResolveUDPAddr("udp", string(tmp)); err == nil {
				logger.Infof("load peer => %v\n", cli)
			}
		}
	} else {
		if cli, err = net.ResolveUDPAddr("udp", cliAddr); err == nil {
			logger.Infof("set peer => %v\n", cli)
		}
	}
	go func() {
		buf := make([]byte, 2000)
		for {
			n, err := iface.Read(buf)
			if err != nil {
				logger.Warningf("tap read error: %v\n", err)
				continue
			}
			if localIP[0] == buf[16] && localIP[1] == buf[17] && localIP[2] == buf[18] && localIP[3] == buf[19] {
				if _, err := iface.Write(buf[:n]); err != nil {
					logger.Warningf("local write error: %v\n", err)
				}
				continue
			}
			if _, err := conn.WriteToUDP(buf[:n], cli); err != nil {
				logger.Warningf("udp write error: %v\n", err)
				continue
			}
		}
	}()
	var lastCli string
	var n int
	data := make([]byte, 2000)
	for {
		n, cli, err = conn.ReadFromUDP(data)
		if err != nil {
			logger.Warning("failed read udp msg, error: " + err.Error())
		}
		dest := toIntIP(data, 16, 17, 18, 19)
		if sess, ok := sessions[dest]; ok && n > 1 {
			if _, err := expose.WriteToUDP(data[:n], sess); err != nil {
				logger.Warningf("session write error: %d %v %v\n", n, data[:n], err)
			}
		} else {
			if _, err := iface.Write(data[:n]); err != nil {
				if data[0] == 0 && n == 1 {
					if lastCli == cli.String() {
						logger.Debugf("client heartbeat => %v\n", cli)
					} else {
						if lastCli == "" {
							logger.Infof("client init => %v\n", cli)
						} else {
							logger.Infof("client change => %v\n", cli)
						}
						lastCli = cli.String()
						if cliAddr == "" {
							ioutil.WriteFile(TmpPeer, []byte(lastCli), 0644)
						}
						sendIptable(cli, iptables)
					}
				} else {
					logger.Warningf("tun write error: %d %v %v\n", n, data[:n], err)
				}
			}
		}
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func toIntIP(packet []byte, offset0 int, offset1 int, offset2 int, offset3 int) (sum uint64) {
	sum = 0
	sum += uint64(packet[offset0]) << 24
	sum += uint64(packet[offset1]) << 16
	sum += uint64(packet[offset2]) << 8
	sum += uint64(packet[offset3])
	return sum
}
