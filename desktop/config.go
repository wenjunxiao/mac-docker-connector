package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/op/go-logging"
	"github.com/songgao/water"
)

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

func loadConfig(iface *water.Interface, init bool) *water.Interface {
	fi, err := os.Open(configFile)
	if err != nil {
		logger.Error("load config failed", err)
		return iface
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
		s := strings.TrimSpace(string(a))
		match := re.FindStringSubmatch(s)
		if match != nil {
			val := match[2]
			switch match[1] {
			case "loglevel":
				if level, err := logging.LogLevel(val); err == nil {
					logging.SetLevel(level, "vpn")
					if leveledBackend != nil {
						leveledBackend.SetLevel(level, "vpn")
					}
				}
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
			default:
				logger.Warningf("unknown action => %s\n", match[1])
			}
		} else if s != "" && !strings.HasPrefix(s, "#") {
			logger.Warningf("invalid config => %s\n", s)
		}
	}
	if init {
		if peer, subnet, err = net.ParseCIDR(addr); err != nil {
			logger.Fatal(err)
		}
		copy([]byte(localIP), []byte(peer.To4()))
		localIP[3]++
		if bind {
			iface = setup(localIP, peer, subnet)
		}
		for k, v := range iptables1 {
			iptables[k] = v
		}
	}
	logger.Debugf("routes %s => %s\n", map2json(routes), map2json(news))
	for key := range routes {
		if val, ok := news[key]; ok {
			routes[key] = val
			delete(news, key)
		} else if bind {
			delRoute(key)
		}
	}
	for key := range news {
		routes[key] = news[key]
		if bind {
			delRoute(key)
			addRoute(key, peer)
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
	return iface
}

func map2json(m interface{}) string {
	if b, err := json.Marshal(m); err == nil {
		return string(b)
	}
	return ""
}

func appendConfig(data []byte) {
	fd, err := os.OpenFile(configFile, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	fd.WriteString("\n")
	fd.Write(data)
	fd.Close()
}

func sendConfig() {
	logger.Infof("send config to => %s:%d\n", host, port)
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	var data bytes.Buffer
	if len(os.Args) > 2 {
		data.WriteByte(1)
		data.WriteString(strings.Join(os.Args[2:], " "))
		conn.Write(data.Bytes())
	} else {
		reader := bufio.NewReader(os.Stdin)
		for {
			line, hasMore, err := reader.ReadLine()
			if err != nil {
				break
			}
			data.Reset()
			data.WriteByte(1)
			data.Write(line)
			conn.Write(data.Bytes())
			if !hasMore {
				break
			}
		}
	}
}

func clearRoutes() {
	for key := range routes {
		delRoute(key)
	}
}
