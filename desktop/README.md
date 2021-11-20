# docker-connector

  Accept connection from [desktop-connector](../docker) in docker, and route container's ip to it.
  Also can expose the access capabilities of Docker containers to others who use [docker-accessor](../accessor)

## Install

  You can install from [brew](https://github.com/wenjunxiao/homebrew-brew)
```bash
$ brew install wenjunxiao/brew/docker-connector
```
  Or download the latest version directly from [release](https://github.com/wenjunxiao/mac-docker-connector/releases)
```bash
# change the version to latest version
$ curl -sSL -o- https://github.com/wenjunxiao/mac-docker-connector/releases/download/v1.0/docker-connector-mac.tar.gz | tar -zxf - -C /usr/local/bin/
```

## Usage

  If install by brew, just start as a service.
```bash
$ sudo brew services start docker-connector
```
  Add routes which container subnets you want to access for the first time,
  and you can add or delete later.
  You can add all bridge subnet by `docker network ls --filter driver=bridge`
```bash
$ docker network ls --filter driver=bridge --format "{{.ID}}" | xargs docker network inspect --format "route {{range .IPAM.Config}}{{.Subnet}}{{end}}" >> /usr/local/etc/docker-connector.conf
```
  Or just add specified subnet route you like
```bash
$ cat <<EOF >> /usr/local/etc/docker-connector.conf
route 172.100.0.0/16
EOF
```

  Start with the specified configuration file
```bash
$ sudo ls # cache sudo password
$ nohup sudo ./docker-connector -config /usr/local/etc/docker-connector.conf &
```

### Expose access

  You can expose the containers to others, so that they can access the network you built in docker.
  Add expose listen address and access tokens.
```bash
$ cat <<EOF >> /usr/local/etc/docker-connector.conf
expose 0.0.0.0:2512
token user1 192.168.251.3
token user2 192.168.251.4
EOF
```
  And append `expose` the route which you want to expose to others.
```conf
route 172.100.0.0/16 expose
```

  For test, you can turn on `pong` to intercept ping requests(only IPv4)
```bash
$ cat <<EOF >> /usr/local/etc/docker-connector.conf
pong on
EOF
```

## Compile

```bash
$ go env -w GOPROXY=https://goproxy.cn,direct
$ go build -tags netgo -o docker-connector main.go
```

## Publish

  Publish this to [Homebrew](https://brew.sh/) for easy installation.

### Release

  Build and make a tarball for Mac
```bash
$ go build -tags netgo -o ./build/darwin/docker-connector .
$ tar -czf build/docker-connector-darwin.tar.gz -C ./build/darwin docker-connector
$ shasum -a 256 build/docker-connector-darwin.tar.gz | awk '{print $1}' > build/docker-connector-darwin-sha256.txt
```
  Build and make a zip for Windows
```bash
$ GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -tags netgo -o ./build/win/x86_64/docker-connector/docker-connector.exe .
$ cat options.conf.template > ./build/win/x86_64/docker-connector/options.conf
$ cp tools/* ./build/win/x86_64/docker-connector/
$ cd ./build/win/x86_64/ && zip -r docker-connector-win-x86_64.zip docker-connector && cd ../../../
$ GOOS=windows GOARCH=386 go build -ldflags "-s -w" -tags netgo -o ./build/win/i386/docker-connector/docker-connector.exe .
$ cat options.conf.template > ./build/win/i386/docker-connector/options.conf
$ cp tools/* ./build/win/i386/docker-connector/
$ cd ./build/win/i386/ && zip -r docker-connector-win-i386.zip docker-connector && cd ../../../
```
  Upload the tarball to [Releases](https://github.com/wenjunxiao/mac-docker-connector/releases)

### Homebrew

  Create a ruby repository named `homebrew-brew`, which must start with `homebrew-`.
  Clone it and add formula named [docker-connector.rb](https://github.com/wenjunxiao/homebrew-brew/blob/master/Formula/docker-connector.rb) in `Formula` 
```bash
$ git clone https://github.com/wenjunxiao/homebrew-brew
$ cd homebrew-brew
$ mkdir Formula && cd Formula
$ cat <<EOF > docker-connector.rb
class DockerConnector < Formula
  url https://github.com/wenjunxiao/mac-docker-connector/releases/download/x.x.x/docker-connector-mac.tar.gz
  sha256 ...
  version ...
  def install
    bin.install "docker-connector"
  end
  def plist
    <<~EOS
      ...
    EOS
  end
end
EOF
$ cd ../
$ git add . && git commit -m "..."
$ git push origin master
```
  You can install by brew.
```bash
$ brew install wenjunxiao/brew/docker-connector
```
  In addition to github, it can be stored in other warehouses,
  and other protocols can also be used. Such as [gitee.com](https://gitee.com/wenjunxiao/homebrew-brew).
  You need to specify the full path when installing
```bash
$ brew tap wenjunxiao/brew https://gitee.com/wenjunxiao/homebrew-brew
$ brew install docker-connector
```
  If it has already been tapped, you can change remote 
```bash
$ cd `brew --repo`/Library/Taps/wenjunxiao/homebrew-brew
$ git remote set-url origin https://gitee.com/wenjunxiao/homebrew-brew.git
$ brew install docker-connector
```

## Dev

  Run `main.go` without default config in debug mode.
```bash
$ sudo go run main.go -port 2521 -addr 192.168.252.1/24 -cli false
```

## References

* [Internet Protocol](https://www.ietf.org/rfc/rfc791)