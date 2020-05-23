package main

import (
	"bufio"
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
	peer   net.IP
	// MTU maximum transmission unit
	MTU        = 1400
	host       = "127.0.0.1"
	port       = 2511
	addr       = "192.168.251.1/24"
	configFile = ""
	watch      = true
	routes     = make(map[string]bool)
	logLevel   = "INFO"
)

func init() {
	logging.SetLevel(logging.INFO, "vpn")
	logger = logging.MustGetLogger("vpn")
	flag.IntVar(&MTU, "mtu", MTU, "mtu")
	flag.StringVar(&host, "host", host, "udp host")
	flag.IntVar(&port, "port", port, "udp port")
	flag.StringVar(&addr, "addr", addr, "address")
	flag.StringVar(&configFile, "config", configFile, "config file")
	flag.BoolVar(&watch, "watch", watch, "watch config file")
	flag.StringVar(&logLevel, "log-level", logLevel, "log level")
}

func runCmd(args string) error {
	argv := strings.Split(args, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	logger.Infof("command => %s", args)
	return cmd.Run()
}

func loadConfig(iface *water.Interface, init bool) {
	fi, err := os.Open(configFile)
	if err != nil {
		return
	}
	defer fi.Close()
	re := regexp.MustCompile(`^\s*(\w+\S+)(?:\s+(.*))?$`)
	news := make(map[string]bool)
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
				news[val] = true
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
			}
		}
	}
	if init {
		if peer, _, err = net.ParseCIDR(addr); err != nil {
			logger.Fatal(err)
		}
		ip := net.IP(make([]byte, 4))
		copy([]byte(ip), []byte(peer.To4()))
		ip[3]++
		args := fmt.Sprintf("%s %s inet %s %s netmask 255.255.255.255 up", "ifconfig", iface.Name(), ip, peer)
		if err = runCmd(args); err != nil {
			logger.Fatal(err)
		}
	}
	for key := range routes {
		if _, ok := news[key]; !ok {
			args := fmt.Sprintf("route -n delete -net %s", key)
			runCmd(args)
		} else {
			delete(news, key)
		}
	}
	for key := range news {
		runCmd(fmt.Sprintf("route -n delete -net %s", key))
		args := fmt.Sprintf("route -n add -net %s %s", key, peer)
		if err = runCmd(args); err != nil {
			logger.Warning(err)
		}
	}
	news = nil
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
			}
		}
	} else {
		if peer, _, err = net.ParseCIDR(addr); err != nil {
			logger.Fatal(err)
		}
		ip := net.IP(make([]byte, 4))
		copy([]byte(ip), []byte(peer.To4()))
		ip[3]++
		args := fmt.Sprintf("%s %s inet %s %s netmask 255.255.255.255 up", "ifconfig", iface.Name(), ip, peer)
		if err = runCmd(args); err != nil {
			logger.Fatal(err)
		}
	}
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		logger.Fatalf("invalid address => %s:%d", host, port)
	}
	// 监听
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		logger.Fatalf("failed to listen %s:%d => %s", host, port, err.Error())
		return
	}
	defer conn.Close()
	logger.Infof("listen => %v\n", conn.LocalAddr())
	var cli *net.UDPAddr
	if tmp, err := ioutil.ReadFile(TmpPeer); err == nil {
		if cli, err = net.ResolveUDPAddr("udp", string(tmp)); err == nil {
			logger.Infof("load peer => %v\n", cli)
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
			if _, err := conn.WriteToUDP(buf[:n], cli); err != nil {
				logger.Warningf("udp write error: %v\n", err)
				continue
			}
		}
	}()
	var n int
	data := make([]byte, 2000)
	for {
		n, cli, err = conn.ReadFromUDP(data)
		if err != nil {
			logger.Warning("failed read udp msg, error: " + err.Error())
		}
		if _, err := iface.Write(data[:n]); err != nil {
			if data[0] == 0 && n == 1 {
				logger.Debugf("client init => %v\n", cli)
				ioutil.WriteFile(TmpPeer, []byte(cli.String()), 0644)
			} else {
				logger.Warningf("tun write error: %d %v %v\n", n, data[:n], err)
			}
		}
	}
}
