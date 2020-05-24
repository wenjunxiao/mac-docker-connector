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

## Usage

  Install mac client of mac-docker-connector.
```bash
$ brew tap wenjunxiao/docker-connector
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

  Install docker front of mac-docker-connector
```bash
$ docker pull origin wenjunxiao/mac-docker-connector
```

  Start the docker front. The network must be host, and add `NET_ADMIN` capability.

```bash
$ docker run -it -d --restart always --net host --cap-add NET_ADMIN --name connector mac-docker-connector
```