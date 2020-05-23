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

