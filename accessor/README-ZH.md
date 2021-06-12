[English](README.md) | [中文简体](README-ZH.md)

# docker-accessor

  连接到macOS上的`docker-connector`，以便于访问macOS导出的容器

## 安装

### Windows

  首先需要从[OpenVPN](https://community.openvpn.net/openvpn/wiki/ManagingWindowsTAPDrivers)下载安装tap驱动[tap-windows](http://build.openvpn.net/downloads/releases/)。
  下载最新版本`http://build.openvpn.net/downloads/releases/latest/tap-windows-latest-stable.exe`并安装.
  然后从[Releases](https://github.com/wenjunxiao/mac-docker-connector/releases)下载最新的`docker-accessor`

### MacOS

  通过brew安装
```bash
$ brew install wenjunxiao/brew/docker-accessor
```

### Linux

  从[Releases](https://github.com/wenjunxiao/mac-docker-connector/releases)下载`docker-accessor` 
```bash
$ curl -L -o- https://github.com/wenjunxiao/mac-docker-connector/releases/download/v2.0/docker-accessor-linux.tar.gz | tar -xzf - -C /usr/local/bin
```

## 使用

  通过macOS上的`docker-connector`导出的地址和端口，以及分配的令牌启动`docker-accessor`
```bash
$ docker-accessor -remote 192.168.1.100:2512 -token my-token
```
  如果本地存在相同的某些子网，也可以排出对应的子网以避免和本地的子网冲突
```bash
$ docker-accessor -remote 192.168.1.100:2512 -token my-token -exclude 172.1.0.0/24,172.2.0.0/24
```

## Compile

### Windows

```bash
$ GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -tags netgo -o ./build/win/x86_64/docker-accessor.exe .
$ zip -j docker-accessor-win-x86_64.zip ./build/win/x86_64/docker-accessor.exe
$ GOOS=windows GOARCH=386 go build -ldflags "-s -w" -tags netgo -o ./build/win/i686/docker-accessor.exe .
$ zip -j docker-accessor-win-i686.zip ./build/win/i686/docker-accessor.exe
```

### Linux

```bash
$ GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -tags netgo -o ./build/linux/docker-accessor .
$ tar -czf docker-accessor-linux.tar.gz -C ./build/linux docker-accessor
```

### MacOS

```bash
$ GOOS=darwin go build -ldflags "-s -w" -tags netgo -o ./build/darwin/docker-accessor .
$ tar -czf docker-accessor-darwin.tar.gz -C ./build/darwin docker-accessor
$ shasum -a 256 docker-accessor-darwin.tar.gz | awk '{print $1}' > docker-accessor-darwin-sha256.txt
```
