[English](README.md) | [中文简体](README-ZH.md)

# docker-accessor

  Connect to `docker-connector` in macOS and access the exposed container of docker.

## Install

### Windows

  Need to install tap driver [tap-windows](http://build.openvpn.net/downloads/releases/) from [OpenVPN](https://community.openvpn.net/openvpn/wiki/ManagingWindowsTAPDrivers).
  Download the latest version `http://build.openvpn.net/downloads/releases/latest/tap-windows-latest-stable.exe` and install.
  Download `docker-accessor` from [Releases](https://github.com/wenjunxiao/mac-docker-connector/releases)

### MacOS

  Install by brew.

```bash
$ brew install wenjunxiao/brew/docker-accessor
```

### Linux

  Download `docker-accessor` from [Releases](https://github.com/wenjunxiao/mac-docker-connector/releases)
```bash
$ curl -L -o- https://github.com/wenjunxiao/mac-docker-connector/releases/download/v2.0/docker-accessor-linux.tar.gz | tar -xzf - -C /usr/local/bin
```

## Usage

  Run `docker-accessor` with remote `docker-connector` expose address and assigned token.
```bash
$ docker-accessor -remote 192.168.1.100:2512 -token my-token
```
  Also can exclude some network of docker to prevent conflict with local.
```bash
$ docker-accessor -remote 192.168.1.100:2512 -token my-token -exclude 172.1.0.0/24,172.2.0.0/24
```

## Compile

### Windows

```bash
$ GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -tags netgo -o ./build/win/x86_64/docker-accessor.exe .
$ zip -j build/docker-accessor-win-x86_64.zip ./build/win/x86_64/docker-accessor.exe
$ GOOS=windows GOARCH=386 go build -ldflags "-s -w" -tags netgo -o ./build/win/i386/docker-accessor.exe .
$ zip -j build/docker-accessor-win-i386.zip ./build/win/i386/docker-accessor.exe
```

### Linux

```bash
$ GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -tags netgo -o ./build/linux/docker-accessor .
$ tar -czf build/docker-accessor-linux.tar.gz -C ./build/linux docker-accessor
```

### MacOS

```bash
$ GOOS=darwin go build -ldflags "-s -w" -tags netgo -o ./build/darwin/docker-accessor .
$ tar -czf build/docker-accessor-darwin.tar.gz -C ./build/darwin docker-accessor
$ shasum -a 256 build/docker-accessor-darwin.tar.gz | awk '{print $1}' > build/docker-accessor-darwin-sha256.txt
```
