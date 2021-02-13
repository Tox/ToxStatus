# ToxStatus ![build](https://github.com/Tox/ToxStatus/workflows/build/badge.svg)

Status page written in Go that keeps track of Tox bootstrap nodes.

The entire codebase is licensed under [AGPL](LICENSE) unless stated otherwise.

## Screenshots

![](https://alexbakker.me/u/hbxed4cdsk.png)

![](https://alexbakker.me/u/a3jyllwn9v.png)

## Tool

Besides being a full status page, ToxStatus can also be used as a command line tool to quickly check the status of a node.

```none
~> ./ToxStatus --help
Usage of ./ToxStatus:
  -ip string
        ip address to probe, ipv4 and ipv6 are both supported (default "127.0.0.1")
  -key string
        public key of the node
  -net string
        network type, either 'udp' or 'tcp' (default "udp")
  -port int
        port to probe (default 33445)
```
