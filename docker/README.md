# mac-receiver

  Connect to `docker-connector` in macOS and route the request from macOS to the container correctly.

## Usage

```bash
$ docker run -it -d --net host --cap-add NET_ADMIN --name mac-receiver mac-docker-connector
```

## Compile

### Docker

```bash
$ docker build -t mac-docker-connector .
```

### Local
  Local compile
```bash
$ GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -tags netgo -o mac-receiver main.go
```

## Dev

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
> go build -ldflags "-s -w" -tags netgo -o mac-receiver main.go
```
