package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/songgao/water"
)

var (
	debug   = false
	remote  = ""
	token   = ""
	exclude = ""
	clears  = make(map[string]bool)
)

func init() {
	flag.BoolVar(&debug, "debug", debug, "Provide debug info")
	flag.StringVar(&remote, "remote", remote, "remote address")
	flag.StringVar(&token, "token", token, "assigned token")
	flag.StringVar(&exclude, "exclude", token, "exclude network")
}

func _runCmd(args string, output bool) (string, error) {
	argv := strings.Split(args, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	out, err := cmd.CombinedOutput()
	if out != nil {
		outStr := string(out)
		if output {
			fmt.Printf("command => %s %v\n", args, outStr)
		} else {
			fmt.Printf("command => %s\n", args)
		}
		return outStr, err
	}
	return "", err
}

func runCmd(args string) (string, error) {
	return _runCmd(args, false)
}

func runOutCmd(args string) (string, error) {
	return _runCmd(args, true)
}

func main() {
	flag.Parse()
	udpAddr, err := net.ResolveUDPAddr("udp", remote)
	if err != nil {
		fmt.Printf("invalid address => %s\n", remote)
		os.Exit(1)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		fmt.Printf("failed to dial %s => %s\n", remote, err.Error())
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("local => %s\n", conn.LocalAddr())
	fmt.Printf("remote => %s\n", conn.RemoteAddr())
	conn.Write([]byte(fmt.Sprintf("%s%s", string([]byte{0}), token)))
	data := make([]byte, 2000)
	ready := make(chan bool)
	exiting := false
	var iface *water.Interface
	go func() {
		<-ready
		buf := make([]byte, 2000)
		for {
			n, err := iface.Read(buf)
			if err != nil {
				if exiting {
					break
				}
				fmt.Printf("tun read error: %v\n", err)
				time.Sleep(time.Second)
				continue
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				fmt.Printf("udp write error: %v\n", err)
			}
		}
	}()
	logged := false
	go func() {
		for {
			n, err := conn.Read(data)
			if err != nil {
				if exiting {
					break
				}
				fmt.Printf("failed read udp msg: %d %v\n", n, err.Error())
				if n == 0 {
					time.Sleep(time.Second)
					continue
				}
			}
			if logged {
				if _, err := iface.Write(data[:n]); err != nil {
					if data[0] == 2 && n == 1 { // 未认证
						logged = false
						fmt.Println("relogin")
						conn.Write([]byte(fmt.Sprintf("%s%s", string([]byte{1}), token)))
					} else if n > 0 {
						fmt.Printf("tun write error: %v\n", err)
					}
				}
			} else {
				if data[0] == 1 {
					logged = true
					fmt.Println("logged")
					if iface != nil {
						for cm := range clears {
							runCmd(cm)
							delete(clears, cm)
						}
						iface.Close()
						time.Sleep(time.Second)
						iface = setup(strings.Split(string(data[1:n]), ","))
					} else {
						iface = setup(strings.Split(string(data[1:n]), ","))
						ready <- true
					}
				} else if data[0] == 2 { // 未认证
					logged = false
					fmt.Println("relogin")
					conn.Write([]byte(fmt.Sprintf("%s%s", string([]byte{1}), token)))
				}
			}
		}
	}()
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)
	s := <-c
	fmt.Println("exit signal =>", s)
	exiting = true
	for cm := range clears {
		runCmd(cm)
	}
	iface.Close()
}
