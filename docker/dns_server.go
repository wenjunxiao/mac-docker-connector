package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

type DNSServer struct {
	udp     *net.UDPConn
	a       map[string][4]byte
	ptr     map[string]string
	tmp     map[string]byte
	up      *net.UDPConn
	pending map[string][]*net.UDPAddr
}

func NewDnsServer() *DNSServer {
	return &DNSServer{
		a:   make(map[string][4]byte),
		ptr: make(map[string]string),
	}
}

func (s *DNSServer) Start(ip net.IP) error {
	if s.udp != nil {
		return nil
	}
	go s.run(ip)
	return nil
}

func (s *DNSServer) run(ip net.IP) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: ip, Port: 53})
	if err != nil {
		panic(err)
	}
	fmt.Printf("dns listen => %v\n", conn.LocalAddr())
	defer conn.Close()
	s.udp = conn
	for {
		buf := make([]byte, 512)
		_, addr, _ := conn.ReadFromUDP(buf)
		var msg dnsmessage.Message
		if err := msg.Unpack(buf); err != nil {
			fmt.Println(err)
			continue
		}
		go s.serve(addr, conn, msg)
	}
}

func (s *DNSServer) Add(record string) {
	argv := strings.Split(record, " ")
	if len(argv) < 2 {
		return
	}
	ipv4 := net.ParseIP(argv[0]).To4()
	for i := 1; i < len(argv); i++ {
		s.a[argv[i]+"."] = [4]byte{ipv4[0], ipv4[1], ipv4[2], ipv4[3]}
		if s.tmp != nil {
			s.tmp[argv[i]+"."] = 1
		}
	}
	s.ptr[argv[0]+".in-addr.arpa."] = argv[1] + "."
	if s.tmp != nil {
		s.tmp[argv[0]+".in-addr.arpa."] = 1
	}
}

func (s *DNSServer) StartClear() {
	s.tmp = make(map[string]byte)
	for k := range s.a {
		s.tmp[k] = 0
	}
	for k := range s.ptr {
		s.tmp[k] = 0
	}
}

func (s *DNSServer) EndClear() {
	if s.tmp != nil {
		for k := range s.tmp {
			if s.tmp[k] == 0 {
				delete(s.a, k)
				delete(s.ptr, k)
			}
			delete(s.tmp, k)
		}
		s.tmp = nil
	}
}

func (s *DNSServer) serve(addr *net.UDPAddr, conn *net.UDPConn, msg dnsmessage.Message) {
	if len(msg.Questions) < 1 {
		return
	}
	question := msg.Questions[0]
	var (
		queryTypeStr = question.Type.String()
		queryNameStr = question.Name.String()
		queryType    = question.Type
		queryName, _ = dnsmessage.NewName(queryNameStr)
	)
	var resource dnsmessage.Resource
	switch queryType {
	case dnsmessage.TypeAAAA:
		fallthrough
	case dnsmessage.TypeA:
		if rst, ok := s.a[queryNameStr]; ok {
			resource = newAResource(queryName, rst)
		} else {
			fmt.Printf("not fount A record queryName: [%s] \n", queryNameStr)
			s.redirect(addr, queryName.String(), msg)
			return
		}
	case dnsmessage.TypePTR:
		if rst, ok := s.ptr[queryName.String()]; ok {
			resource = newPTRResource(queryName, rst)
		} else {
			fmt.Printf("not fount PTR record queryName: [%s] \n", queryNameStr)
			s.redirect(addr, queryName.String(), msg)
			return
		}
	default:
		fmt.Printf("not support dns queryType: [%s] \n", queryTypeStr)
		return
	}
	msg.Response = true
	msg.Answers = append(msg.Answers, resource)
	s.response(addr, msg)
}

func readNameServer() string {
	fi, err := os.Open("/etc/resolv.conf")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return ""
	}
	defer fi.Close()
	re := regexp.MustCompile(`^\s*nameserver\s+(\d+[\d.]+)`)
	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		match := re.FindStringSubmatch(string(a))
		if match != nil {
			return match[1]
		}
	}
	return ""
}

func (s *DNSServer) redirect(addr *net.UDPAddr, name string, msg dnsmessage.Message) {
	packed, err := msg.Pack()
	if err != nil {
		fmt.Println(err)
		return
	}
	if s.up == nil {
		ip := readNameServer()
		if ip == "" {
			s.response(addr, msg)
			return
		}
		fmt.Printf("dns upstream is %v\n", ip)
		s.up, err = net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP(ip), Port: 53})
		if err != nil {
			s.response(addr, msg)
			return
		}
		s.pending = make(map[string][]*net.UDPAddr)
		go s.upstream(s.up)
	}
	s.pending[name] = append(s.pending[name], addr)
	if _, err := s.up.Write(packed); err != nil {
		fmt.Println(err)
		s.up = nil
		return
	}
}

func (s *DNSServer) upstream(up *net.UDPConn) {
	buf := make([]byte, 512)
	for {
		_, err := up.Read(buf)
		if err != nil {
			fmt.Println("read upstream msg, error: " + err.Error())
			break
		}
		var rsp dnsmessage.Message
		if err := rsp.Unpack(buf); err != nil {
			fmt.Println(err)
			continue
		}
		if len(rsp.Questions) < 1 {
			continue
		}
		question := rsp.Questions[0]
		name := question.Name.String()
		addrs := s.pending[name]
		delete(s.pending, name)
		for _, addr := range addrs {
			s.response(addr, rsp)
		}
	}
}

func (s *DNSServer) response(addr *net.UDPAddr, msg dnsmessage.Message) {
	packed, err := msg.Pack()
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, err := s.udp.WriteToUDP(packed, addr); err != nil {
		fmt.Println(err)
	}
}

func newAResource(query dnsmessage.Name, a [4]byte) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  query,
			Class: dnsmessage.ClassINET,
			TTL:   600,
		},
		Body: &dnsmessage.AResource{
			A: a,
		},
	}
}

func newPTRResource(query dnsmessage.Name, ptr string) dnsmessage.Resource {
	name, _ := dnsmessage.NewName(ptr)
	return dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  query,
			Class: dnsmessage.ClassINET,
		},
		Body: &dnsmessage.PTRResource{
			PTR: name,
		},
	}
}
