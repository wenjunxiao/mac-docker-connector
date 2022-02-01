package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kardianos/service"
	"github.com/op/go-logging"
)

var (
	logger *logging.Logger
	conn   *net.UDPConn
	cli    *net.UDPAddr
	peer   net.IP
	expose *net.UDPConn
	subnet *net.IPNet
	// TmpPeer peer tmp file
	TmpPeer = ""
	// MTU maximum transmission unit
	MTU            = 1400
	host           = "127.0.0.1"
	port           = 2511
	addr           = "192.168.251.1/24"
	configFile     = ""
	watch          = true
	routes         = make(map[string]bool)
	tokens         = make(map[string]string)
	iptables       = make(map[string]bool)
	logLevel       = "INFO"
	sessions       = make(map[uint64]*net.UDPAddr)
	localIP        = net.IP(make([]byte, 4))
	pong           = false
	cliAddr        = ""
	bind           = true
	logfile        = ""
	leveledBackend logging.LeveledBackend
	hosts      	   = ""
)

func init() {
	TmpPeer = filepath.Join(os.TempDir(), "desktop-docker-connector.peer")
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
	flag.BoolVar(&bind, "bind", bind, "bind to interface")
	flag.StringVar(&logfile, "log-file", logfile, "log file")
}

func runCmd(format string, a ...interface{}) error {
	args := fmt.Sprintf(format, a...)
	argv := strings.Split(args, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	logger.Infof("command => %s", args)
	return cmd.Run()
}

func runOutCmd(format string, a ...interface{}) (string, error) {
	args := fmt.Sprintf(format, a...)
	argv := strings.Split(args, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	out, err := cmd.CombinedOutput()
	logger.Infof("command => %s", args)
	if out != nil {
		outStr := string(out)
		return outStr, err
	}
	return "", err
}

func main() {
	flag.Parse()
	cfg := &service.Config{
		Name:        "DesktopDockerConnector",
		DisplayName: "Desktop Docker Connector",
		Description: "Connect Desktop and Docker",
	}
	if len(os.Args) > 1 {
		cfg.Arguments = os.Args[2:]
	}
	s, err := service.New(&Connector{}, cfg)
	if err != nil {
		logger.Fatal(err)
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			if err := s.Install(); err != nil {
				logger.Fatal(err)
			}
			logger.Info("Install Service Success!")
			return
		case "uninstall":
			s.Stop()
			if err := s.Uninstall(); err != nil {
				logger.Fatal(err)
			}
			logger.Info("Uninstall Service Success!")
			return
		case "start":
			if err := s.Start(); err != nil {
				logger.Fatal(err)
			}
			logger.Info("Start Service Success!")
			return
		case "stop":
			if err := s.Stop(); err != nil {
				logger.Fatal(err)
			}
			logger.Info("Stop Service Success!")
			return
		case "restart":
			s.Stop()
			if err := s.Start(); err != nil {
				logger.Fatal(err)
			}
			logger.Info("Restart Service Success!")
			return
		case "config":
			sendConfig()
			return
		}
	}
	if err := s.Run(); err != nil {
		logger.Fatal(err)
	}
}
