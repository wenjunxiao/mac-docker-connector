FROM --platform=$BUILDPLATFORM golang:1.13.5-alpine3.10 AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
WORKDIR /build
ENV GOPROXY https://goproxy.cn
ADD . /build/
RUN CGO_ENABLED=0 GOARCH=${TARGETPLATFORM:6:5} GOOS=linux go build -a -o mac-receiver .

FROM alpine:3.10
COPY --from=builder /build/mac-receiver /usr/bin/
CMD [ "mac-receiver" ]