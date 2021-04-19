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

## Multi-Platform

  Build a multi-platform using [buildx](https://docs.docker.com/buildx/working-with-buildx/).
  First, must create target platforms build env and use it.
```bash
$ docker buildx create --platform linux/amd64,linux/arm64/v8 --use
```

### Push

  Build and push to hub directly.
```bash
$ docker buildx build --platform linux/amd64,linux/arm64/v8 -t wenjunxiao/mac-docker-connector:latest . --push
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