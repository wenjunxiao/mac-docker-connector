# desktop-connector

  Connect to `docker-connector` in `macOS`/`Windows` host and route the request from `macOS`/`Windows` host to the container correctly.

## Usage

```bash
$ docker run -it -d --net host --cap-add NET_ADMIN --restart always --name desktop-connector wenjunxiao/desktop-docker-connector
```

## Compile

### Docker

```bash
$ docker build -t desktop-docker-connector .
```

### Local
  Local compile
```bash
$ GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -tags netgo -o desktop-connector .
```

## Dev

### Centos
  Build dev docker image
```bash
$ docker run -it --name centos-go centos:7 bash
> yum install -y epel-release
> yum install go net-tools tcpdump initscripts
> go env -w GOPROXY=https://goproxy.cn,direct
$ docker commit centos-go centos-go
```

  Run with the dev image
```bash
$ docker run -it --cap-add NET_ADMIN -v $PWD:/workspace --workdir /workspace centos-go bash
> # go mod init main
> go env -w GOPROXY=https://goproxy.cn,direct
> go run .
```
  Or build
```bash
$ go build -ldflags "-s -w" -tags netgo -o desktop-connector .
```

### Alpine

```bash
$ docker run -it --cap-add NET_ADMIN --net host -v $PWD:/workspace --workdir /workspace golang:1.13.5-alpine3.10 bash
> go env -w GOPROXY=https://goproxy.cn,direct
> go run .
```
  To install other tools, such as `iptables`, replace apk repository source to avalibale one, such as in china
```bash
sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
```

## Multi-Platform

  Build a multi-platform using [buildx](https://docs.docker.com/buildx/working-with-buildx/).
  First, must create target platforms build env and use it.
```bash
$ docker buildx create --platform linux/amd64,linux/arm64/v8 --use
```

### Push

  Build and push to hub directly.
```bash
$ docker buildx build --platform linux/amd64,linux/arm64/v8 -t wenjunxiao/desktop-docker-connector:latest . --push
```
  
### Local
  In the local development environment, you can use `--load` to save to local mirror,
  but only a single platform can be used at a time.

  Build an example for platform `linux/arm64`, that only prints platform.
```bash
$ cat <<EOF | docker buildx build --platform linux/arm64/v8 -t demo:arm64 - --load
FROM --platform=\$BUILDPLATFORM golang:1.13.5-alpine3.10 AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
WORKDIR /build
RUN echo $'package main\n\
import (\n\
	"fmt"\n\
	"runtime"\n\
)\n\n\
func main() {\n\
  fmt.Printf("runtime GOARCH: %s\\\n", runtime.GOARCH)\n' > main.go \
  && echo "  fmt.Println(\"build on \$BUILDPLATFORM, build for \$TARGETPLATFORM\")" >> main.go \
  && echo "}" >> main.go
RUN cat main.go && CGO_ENABLED=0 GOARCH=\${TARGETPLATFORM:6} GOOS=linux go build -a -o demo .

FROM alpine:3.10
COPY --from=builder /build/ /usr/bin/
EOF
```
  Run with image `demo:arm64`
```bash
$ docker run --rm -it demo:arm64 sh
WARNING: The requested image's platform (linux/arm64) does not match the detected host platform (linux/amd64) and no specific platform was requested
/ # demo
runtime GOARCH: arm64
build on linux/amd64, build for linux/arm64
```
  Run with image `demo:arm64` and platform `linux/arm64`
```bash
$ docker run --rm -it --platform linux/arm64 demo:arm64 sh
/ # demo
runtime GOARCH: arm64
build on linux/amd64, build for linux/arm64
```

## Advance

  Advanced feature depends on `iptables` tool. But all rules of `iptables` will be cleared after restart, so the connector will set all the rules from [desktop](../desktop) after restart.
  
### Connect Two Subnet
  Two sub bridge net created by follow command
```bash
$ docker network create --subnet 172.18.0.0/16 net1
$ docker network create --subnet 172.19.0.0/16 net2
```
  Start two containers using the above IPs respectively, and then ping each other after both started.
```bash
$ docker run --rm -it -e "PS1=`ifconfig eth0 | sed -nr 's/.*inet (addr:)?(([0-9]*\.){3}[0-9]*).*/\2/p'`> " --net static alpine sh
172.18.0.3> ping -c 1 -W 1 172.19.0.2
PING 172.19.0.2 (172.19.0.2): 56 data bytes

--- 172.19.0.2 ping statistics ---
1 packets transmitted, 0 packets received, 100% packet loss
```

```bash
$ docker run --rm -it -e "PS1=`ifconfig eth0 | sed -nr 's/.*inet (addr:)?(([0-9]*\.){3}[0-9]*).*/\2/p'`> " --net test alpine sh
172.19.0.2> ping -c 1 -W 1 172.18.0.3
PING 172.18.0.3 (172.18.0.3): 56 data bytes

--- 172.18.0.3 ping statistics ---
1 packets transmitted, 0 packets received, 100% packet loss
```
  
  If you want the two containers to be able to access each other, you need to enter the connector container and execute the following command
```bash
$ docker exec -it desktop-connector sh
$ docker exec -it desktop-connector sh
> route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         192.168.65.1    0.0.0.0         UG    0      0        0 eth0
127.0.0.0       0.0.0.0         255.0.0.0       U     0      0        0 lo
172.17.0.0      0.0.0.0         255.255.0.0     U     0      0        0 docker0
172.18.0.0      0.0.0.0         255.255.0.0     U     0      0        0 br-bde9105adf96
172.19.0.0      0.0.0.0         255.255.0.0     U     0      0        0 br-b0bc2fea23d4
192.168.65.0    0.0.0.0         255.255.255.0   U     0      0        0 eth0
192.168.65.5    0.0.0.0         255.255.255.255 UH    0      0        0 services1
```
  The iface of subnet `172.18.0.0/16` is `br-bde9105adf96`, and the iface of subnet `172.19.0.0/16` is `br-b0bc2fea23d4`.
  Connect them by following commands in connector container
```bash
$ iptables -I DOCKER-USER -i br-bde9105adf96 -o br-b0bc2fea23d4 -j ACCEPT
$ iptables -I DOCKER-USER -i br-b0bc2fea23d4 -o br-bde9105adf96 -j ACCEPT
```
  Then, ping the two subnet again.
```bash
172.18.0.3> ping -c 1 -W 1 172.19.0.2
PING 172.19.0.2 (172.19.0.2): 56 data bytes
64 bytes from 172.19.0.2: seq=0 ttl=63 time=0.227 ms

--- 172.19.0.2 ping statistics ---
1 packets transmitted, 1 packets received, 0% packet loss
round-trip min/avg/max = 0.227/0.227/0.227 ms
```
```bash
172.19.0.2> ping -c 1 -W 1 172.18.0.3
PING 172.18.0.3 (172.18.0.3): 56 data bytes
64 bytes from 172.18.0.3: seq=0 ttl=63 time=0.187 ms

--- 172.18.0.3 ping statistics ---
1 packets transmitted, 1 packets received, 0% packet loss
round-trip min/avg/max = 0.187/0.187/0.187 ms
```

### DNS Resolve

  The connector container provider a simple dns server, which only resolve some special domain.
  Route the dns request to connector
```bash
$ iptables -t nat -I PREROUTING -p udp --dport 53 -m string --algo bm --string local -j DNAT --to-destination 192.168.251.1
```
  Show the rules
```bash
$ iptables -t nat -L PREROUTING -vn --line-number
```
  Delete the rule
```bash
$ iptables -t nat -D PREROUTING <num>
```
