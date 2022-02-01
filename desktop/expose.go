package main

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

func packetIP(data []byte) net.IP {
	return net.IPv4(data[16], data[17], data[18], data[19])
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
					logger.Debugf("not supported")
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
