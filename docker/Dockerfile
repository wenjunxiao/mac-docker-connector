FROM --platform=$BUILDPLATFORM golang:1.13.5-alpine3.10 AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
WORKDIR /build
ENV GOPROXY https://goproxy.cn
ADD . /build/
RUN CGO_ENABLED=0 GOARCH=${TARGETPLATFORM:6:5} GOOS=linux go build -ldflags "-s -w" -tags netgo -o desktop-connector .

FROM alpine:3.10
RUN  apk add --no-cache iptables && rm -rf /var/cache/apk/*
COPY --from=builder /build/desktop-connector /usr/bin/
CMD [ "desktop-connector" ]