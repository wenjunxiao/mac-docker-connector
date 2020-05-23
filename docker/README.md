# mac-receiver

  Accept macOS connection request and route it to the container correctly.

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

```bash
$ go mod init main
$ go env -w GOPROXY=https://goproxy.cn,direct
```