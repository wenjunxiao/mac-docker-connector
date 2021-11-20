package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kardianos/service"
	"github.com/op/go-logging"
	"github.com/songgao/water"
)

type Connector struct {
	iface *water.Interface
	stop  bool
}

func (c *Connector) Start(s service.Service) error {
	c.stop = false
	go c.run()
	return nil
}

func (c *Connector) Stop(s service.Service) error {
	c.stop = true
	go func() {
		clearRoutes()
		if conn != nil {
			conn.Close()
		}
		if c.iface != nil {
			c.iface.Close()
		}
	}()
	return nil
}

func (c *Connector) run() {
	flag.Parse()
	if level, err := logging.LogLevel(logLevel); err == nil {
		logging.SetLevel(level, "vpn")
	}
	if logfile != "" {
		if !filepath.IsAbs(logfile) {
			path, err := filepath.Abs(os.Args[0])
			if err == nil {
				logfile = filepath.Join(path, "..", logfile)
			}
		}
		file, err := os.OpenFile(logfile, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0660)
		if err == nil {
			backend := logging.NewLogBackend(file, "", log.LstdFlags)
			leveledBackend = logging.AddModuleLevel(backend)
			logger.SetBackend(leveledBackend)
		}
	}
	if configFile != "" && !filepath.IsAbs(configFile) {
		path, err := filepath.Abs(os.Args[0])
		if err == nil {
			configFile = filepath.Join(path, "..", configFile)
		}
		logger.Infof("config file => %v\n", configFile)
	}
	var iface *water.Interface
	if _, err := os.Stat(configFile); err == nil {
		logger.Infof("load config(%v) => %s\n", watch, configFile)
		iface = loadConfig(iface, true)
		if watch {
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				logger.Fatal(err)
			}
			var timer *time.Timer
			defer watcher.Close()
			loader := func() {
				timer = nil
				loadConfig(iface, false)
			}
			go func() {
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						if event.Op&fsnotify.Write == fsnotify.Write {
							logger.Debugf("config file changed => %s\n", configFile)
							if timer != nil {
								timer.Stop()
							}
							timer = time.AfterFunc(time.Duration(2)*time.Second, loader)
						} else if event.Op&fsnotify.Rename == fsnotify.Rename {
							logger.Debugf("config file renamed => %s\n", event.Name)
							if timer != nil {
								timer.Stop()
							}
							timer = time.AfterFunc(time.Duration(2)*time.Second, loader)
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
		if bind {
			iface = setup(localIP, peer, subnet)
		}
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
	c.iface = iface
	go func() {
		if iface == nil {
			logger.Info("not bind to interface")
			return
		}
		buf := make([]byte, 2000)
		for {
			n, err := iface.Read(buf)
			if err != nil {
				if c.stop {
					break
				}
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
				if cli != nil {
					logger.Warningf("udp write error: %v\n", err)
				}
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
			if c.stop {
				break
			}
			logger.Warning("failed read udp msg, error: " + err.Error())
		}
		dest := toIntIP(data, 16, 17, 18, 19)
		if sess, ok := sessions[dest]; ok && n > 1 {
			if _, err := expose.WriteToUDP(data[:n], sess); err != nil {
				logger.Warningf("session write error: %d %v %v\n", n, data[:n], err)
			}
		} else if bind {
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
				} else if data[0] == 1 && n > 1 {
					appendConfig(data[1:n])
				} else {
					logger.Warningf("tun write error: %d %v %v\n", n, data[:n], err)
				}
			}
		} else if data[0] == 1 && n > 1 {
			appendConfig(data[1:n])
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
