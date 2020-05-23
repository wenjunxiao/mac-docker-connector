# docker-connector

  Connect to mac-receiver in docker, and route container's ip to it.

## Usage

  Add routes which container subnets you want to access for the first time,
  and you can add or delete later.
  You can add all bridge subnet by `docker network ls --filter driver=bridge`
```bash
$ docker network ls --filter driver=bridge --format "{{.ID}}" | xargs docker network inspect --format "route {{range .IPAM.Config}}{{.Subnet}}{{end}}" >> options.conf
```
  Or just add specified subnet route you like
```bash
$ cat <<EOF >> options.conf
route 172.100.0.0/16
EOF
```

  Start with the specified configuration file
```bash
$ sudo ls # cache sudo password
$ nohup sudo ./docker-connector -config options.conf &
```

## Compile

```bash
$ go env -w GOPROXY=https://goproxy.cn,direct
$ go build -tags netgo -o docker-connector main.go
```
