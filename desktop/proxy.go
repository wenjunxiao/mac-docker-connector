package main

import (
	"fmt"
	"io"
	"net"
	"strings"
)

var (
	proxyServer *ProxyServer
)

type ProxyServer struct {
	tcp map[string]net.Listener
	tmp map[string]byte
}

func NewProxyServer() *ProxyServer {
	return &ProxyServer{
		tcp: make(map[string]net.Listener),
	}
}

func GetProxyServer() *ProxyServer {
	if proxyServer == nil {
		proxyServer = NewProxyServer()
	}
	return proxyServer
}

func (s *ProxyServer) StartClear() {
	s.tmp = make(map[string]byte)
	for k := range s.tcp {
		s.tmp[k] = 0
	}
}

func (s *ProxyServer) EndClear() {
	if s.tmp != nil {
		for k := range s.tmp {
			if s.tmp[k] == 0 {
				logger.Infof("close proxy: %s\n", k)
				s.tcp[k].Close()
				delete(s.tcp, k)
			}
			delete(s.tmp, k)
		}
		s.tmp = nil
	}
}

func (s *ProxyServer) Add(addr string) {
	argv := strings.Split(addr, ":")
	if len(argv) < 2 {
		argv = []string{"127.0.0.1", argv[0], argv[0]}
	} else if len(argv) < 3 {
		argv = []string{argv[0], argv[1], argv[1]}
	}
	addr = strings.Join(argv, ":")
	if _, ok := s.tcp[addr]; ok {
		return
	}
	s.tcp[addr] = nil
}

func (s *ProxyServer) Start(ip net.IP) {
	for key := range s.tcp {
		if svr, ok := s.tcp[key]; ok {
			if svr == nil {
				argv := strings.Split(key, ":")
				svr, err := net.Listen("tcp", fmt.Sprintf("%s:%s", ip.String(), argv[2]))
				if err != nil {
					logger.Warningf("proxy listen error: %s:%s %v\n", ip.String(), argv[2], err)
					return
				}
				logger.Infof("proxy listen %s:%s\n", ip.String(), argv[2])
				s.tcp[key] = svr
				go func(addr string) {
					for {
						cli, err := svr.Accept()
						if err == nil {
							go s.serve(cli, addr)
						} else if strings.Contains(err.Error(), "closed") {
							break
						} else {
							logger.Warningf("proxy error %v\n", err)
						}
					}
				}(strings.Join(argv[:2], ":"))
			}
		}
	}
}

func (s *ProxyServer) serve(client net.Conn, addr string) {
	defer client.Close()
	remote, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	defer remote.Close()
	go io.Copy(remote, client)
	io.Copy(client, remote)
}
