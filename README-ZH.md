[English](https://github.com/wenjunxiao/mac-docker-connector/blob/master/README.md) | [中文简体](https://github.com/wenjunxiao/mac-docker-connector/blob/master/README-ZH.md)

> 把`mac-docker-connector`重命名为`desktop-docker-connector`是为了同时支持Mac和Widnwos下的Docker
# desktop-docker-connector

  `Docker Desktop for Mac and Windows` 没有提供从宿主的macOS或Windows通过容器IP访问容器的方式。参考[Known limitations, use cases, and workarounds](https://docs.docker.com/docker-for-mac/networking/#i-cannot-ping-my-containers)。通过一个[复杂解决方法](https://pjw.io/articles/2018/04/25/access-to-the-container-network-of-docker-for-mac/)得到灵感，主要方式在宿主的macOS和Docker的Hypervisor之间建立一个VPN
```
+---------------+          +--------------------+
|               |          | Hypervisor/Hyper-V |
| macOS/Windows |          |  +-----------+     |
|               |          |  | Container |     |
|               |   vpn    |  +-----------+     |
|   VPN Client  |<-------->|   VPN Server       |
+---------------+          +--------------------+
```
  但是宿主的macOS无法直接访问Hypervisor，VPN服务容器需要使用`host`以便与Hypervisor在同一网络环境中，必须使用一个转发容器（比如`socat`)导出端口到macOS，然后转发到VPN服务。考虑到VPN连接的双工的，因此我们可以把VPN服务和客户端反转一下，变成下面的结构
```
+---------------+          +--------------------+
|               |          | Hypervisor/Hyper-V |
| macOS/Windows |          |  +-----------+     |
|               |          |  | Container |     |
|               |   vpn    |  +-----------+     |
| VPN Server    |<-------->|   VPN Client       |
+---------------+          +--------------------+
```
  尽管如此, 我们需要做更多额外的工作来使用openvpn，比如证书、配置等。
  这对于只是通过IP访问容器的需求来说，这些工作略显麻烦。
  我们只需要建立一个连接通道，无需证书，也可以无需客户端
```
+---------------+          +--------------------+
|               |          | Hypervisor/Hyper-V |
| macOS/Windows |          |  +-----------+     |
|               |          |  | Container |     |
|               |   udp    |  +-----------+     |
| TUN Server    |<-------->|   TUN Client       |
+---------------+          +--------------------+
```
  鉴于Docker官方文档[Docker and iptables](https://docs.docker.com/network/iptables/)中描述那样,
  两个子网之间的互通性有时也是需要的，因此还可以通过`iptables`来提供两个子网之间的互相连接
```
+-------------------------------+ 
|      Hypervisor/Hyper-V       | 
| +----------+     +----------+ | 
| | subnet 1 |<--->| subnet 2 | |
| +----------+     +----------+ |
+-------------------------------+
```

## 使用

### 宿主机

#### Mac
  先安装Mac端的服务`mac-docker-connector`
```bash
$ brew tap wenjunxiao/brew
$ brew install docker-connector
```

  首次配置通过以下命令把所有Docker所有`bridge`子网放入配置文件，后续的增减可以参考后面的详细配置
```bash
$ docker network ls --filter driver=bridge --format "{{.ID}}" | xargs docker network inspect --format "route {{range .IPAM.Config}}{{.Subnet}}{{end}}" >> "$(brew --prefix)/etc/docker-connector.conf"
```

  启动Mac端的服务
```bash
$ sudo brew services start docker-connector
```

  安装Docker端的容器`mac-docker-connector`
```bash
$ docker pull wenjunxiao/mac-docker-connector
```

#### Windows

  从[Releases](https://github.com/wenjunxiao/desktop-docker-connector/releases)下载 `desktop-docker-connector`然后解压.

  执行以下命令安装服务，把所有需要访问的Bridge子网地址按照`route 172.17.0.0/16`的格式写入`options.conf`
```cmd
$ desktop-docker-connector.exe install -config options.conf
```

  把所有需要访问的Bridge子网地址按照`route 172.17.0.0/16`的格式写入`options.conf`
```conf
route 172.17.0.0/16
```
  可以通过脚本`start-connector.bat`来直接启动连接器，或者把连接器按照以下步骤安装成服务之后启动:
  1. 运行脚本`install-service.bat`安装服务.
  2. 运行脚本`start-service.bat`来启动服务.
  还可以通过运行脚本`stop-service.bat`停止服务以及运行脚本`uninstall-service.bat`卸载服务

### Docker

  启动Docker端的容器，其中网络必须是`host`，并且添加`NET_ADMIN`特性
```bash
$ docker run -it -d --restart always --net host --cap-add NET_ADMIN --name mac-connector wenjunxiao/mac-docker-connector
```

  如果你向导出你自己的容器给其他人，让其他人可以访问你在容器中搭建的服务，其他人必须安装另一个客户端[docker-accessor](./accessor)，同时你必须开启`expose`（这默认是关闭的）和提供访问的令牌(`token`)，
  更详细的配置说明参考配置说明

## 配置说明

  基本的配置选项，通常你不需要修改他们，除非你的环境冲突（比如端口被占用，子网已使用）。
  一旦需要变更，那么Docker容器`mac-docker-connector`也需要使用相同的参数重新启动
* `addr` 虚拟网络地址, 默认 `192.168.251.1/24`（可以修改，但容器端需要同步修改参数）
  ```
  addr 192.168.251.1/24
  ```
* `port` UDP服务监听端口, 默认`2511`（可以修改，但容器端需要同步修改参数）
  ```
  port 2511
  ```
* `mtu` 网络的MTU值，默认`1400`（可以修改，但容器端需要同步修改参数）
  ```
  mtu 1400
  ```
* `host` UDP监听的地址，仅用于Docker容器`mac-docker-connector`连接使用，处于安全和适应移动办公设置成`127.0.0.1`（通常无需修改）
  ```
  host 127.0.0.1
  ```

  动态热加载的配置选项，修改配置文件之后无需启动，立即生效（除非禁用`watch`）,可以在需要的时候随时增减
* `route` 添加一条访问Docker容器子网的的路由，通常在你通过`docker network create --subnet x.x.x.x/mask name`命令创建一个`bridge`子网时需要添加
  ```
  route 172.100.0.0/16
  ```
* `iptables` 插入(`+`)或删除(`-`)一条`iptables`规则，用于两个子网之间互相访问
  ```
  iptables 172.0.1.0+172.0.2.0
  iptables 172.0.3.0-172.0.4.0
  ```
  IP是无掩码子网的地址，通过`+`连接表示插入一条可以互相访问的规则，通过`-`连接表示删除它们之间互相访问的规则
* `expose` 导出你本地的容器给其他人，指定其他人用于连接的开放端口
  ```
  expose 0.0.0.0:2512
  ```
  导出的地址必须是其他人可以通过[docker-accessor](./accessor)访问的地址
* `token` 定义其他人访问你的服务的令牌，以及连接成功之后分配的虚拟网络IP
  ```
  token token-name 192.168.251.3
  ```
  令牌是自定义的字符串，并且在配置文件中唯一，IP则必须是`addr`配置的虚拟网络中有效的IP
