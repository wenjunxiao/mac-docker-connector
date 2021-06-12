[English](https://github.com/wenjunxiao/mac-docker-connector/blob/master/README.md) | [中文简体](https://github.com/wenjunxiao/mac-docker-connector/blob/master/README-ZH.md)

# mac-docker-connector

  `Docker for Mac` does not provide access to container IP from macOS host. 
  Reference [Known limitations, use cases, and workarounds](https://docs.docker.com/docker-for-mac/networking/#i-cannot-ping-my-containers). 
  There is a [complex solution](https://pjw.io/articles/2018/04/25/access-to-the-container-network-of-docker-for-mac/),
  which is also my source of inspiration. The main idea is to build a VPN between the macOS host and the docker virtual machine.
```
+------------+          +-----------------+
|            |          |    Hypervisor   |
|   macOS    |          |  +-----------+  |
|            |          |  | Container |  |
|            |   vpn    |  +-----------+  |
| VPN Client |<-------->|   VPN Server    |
+------------+          +-----------------+
```
  But the macOS host cannot access the container, the vpn port must be exported and forwarded.
  Since the VPN connection is duplex, so we can reverse it.
```
+------------+          +-----------------+
|            |          |    Hypervisor   |
|   macOS    |          |  +-----------+  |
|            |          |  | Container |  |
|            |   vpn    |  +-----------+  |
| VPN Server |<-------->|   VPN Client    |
+------------+          +-----------------+
```
  Even so, we need to do more extra work to use openvpn, such as certificates, configuration, etc.
  All I want is to access the container via IP, why is it so cumbersome. 
  No need for security, multi-clients, or certificates, just connect.
```
+------------+          +-----------------+
|            |          |    Hypervisor   |
|   macOS    |          |  +-----------+  |
|            |          |  | Container |  |
|            |   udp    |  +-----------+  |
| TUN Server |<-------->|   TUN Client    |
+------------+          +-----------------+
```
  In the view of [Docker and iptables](https://docs.docker.com/network/iptables/), 
  this tool also provides the ability of two subnets to access each other.
```
+-------------------------------+ 
|           Hypervisor          | 
| +----------+     +----------+ | 
| | subnet 1 |<--->| subnet 2 | |
| +----------+     +----------+ |
+-------------------------------+
```

## Usage

  Install mac client of mac-docker-connector.
```bash
$ brew tap wenjunxiao/brew
$ brew install docker-connector
```

  Config route of docker network
```bash
$ docker network ls --filter driver=bridge --format "{{.ID}}" | xargs docker network inspect --format "route {{range .IPAM.Config}}{{.Subnet}}{{end}}" >> /usr/local/etc/docker-connector.conf
```

  Start the service
```bash
$ sudo brew services start docker-connector
```

  Install docker front of `mac-docker-connector`
```bash
$ docker pull origin wenjunxiao/mac-docker-connector
```

  Start the docker front. The network must be `host`, and add `NET_ADMIN` capability.

```bash
$ docker run -it -d --restart always --net host --cap-add NET_ADMIN --name mac-connector wenjunxiao/mac-docker-connector
```

  If you want to expose the containers of docker to other pepole, Please reference [docker-accessor](./accessor)

## Configuration

  Basic configuration items, do not need to modify these, unless your environment conflicts,
  if necessary, then the docker container `mac-docker-connector` also needs to be started with the same parameters
* `addr` virtual network address, default `192.168.251.1/24` (change if it conflict)
  ```
  addr 192.168.251.1/24
  ```
* `port` udp listen port, default `2511` (change if it conflict)
  ```
  port 2511
  ```
* `mtu` the MTU of network, default `1400`
  ```
  mtu 1400
  ```
* `host` udp listen host, used to be connected by mac-docker-connector, default `127.0.0.1` for security and adaptation
  ```
  host 127.0.0.1
  ```

  Dynamic hot-loading configuration items can take effect without restarting,
  and need to be added or modified according to your needs.
* `route` Add a route to access the docker container subnet, usually when you create a bridge network by `docker network create --subnet 172.56.72.0/24 app`, run `echo "route 172.56.72.0/24" >> /usr/local/etc/docker-connector.conf` to append route to config file.
  ```
  route 172.56.72.0/24
  ```
* `iptables` Insert(`+`) or delete(`-`) a iptable rule for two subnets to access each other.
  ```
  iptables 172.0.1.0+172.0.2.0
  iptables 172.0.3.0-172.0.4.0
  ```
  The ip is subnet address without mask, and join with `+` to insert a rule, and join with `-` to delete a rule.
* `expose` Expose you docker container to other pepole, default disabled.
  ```
  expose 0.0.0.0:2512
  ```
  the exposed address should be connected by [docker-accessor](./accessor).
  And then add `expose` after then `route` you want to be exposed
  ```
  route 172.100.0.0/16 expose
  ```
* `token` Define the access token and the virtual IP assigned after connection
  ```
  token token-name 192.168.251.3
  ```
  The token name is customized and unique, and the IP must be valid in the virtual network
  defined by `addr`  
