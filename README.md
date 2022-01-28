# p2pfile - Simple P2P file distribution CLI

## 背景

### 应用场景

- 所有节点网络联通的环境下的文件分布式分发。
- 私有网络环境，和互联网隔离。
- 无文件加密传输需求

### 设计限制

- DHT 网络中在这种环境下意义不大，所以不使用 DHT 网络，而是使用自带的集中 Tracker
  - 在第一个测试版本使用纯 DHT 网络，发现其交换效率低于 Tracker.
- 不需要 Daemon 常驻进程，只需要单个二进制文件。
- 无加密设计
- 只支持单个文件分发，不支持文件夹分发。
- 不支持 IPv6。

### 设计目标

- 提供私有网络环境下的文件分布式分发。
- 提供最简化的使用方法，一条命令。

## 命令行设计

```txt
Simple P2P file distribution CLI. For example:

p2pfile serve <FILE_PATH1> <FILE_PATH2> ...
p2pfile download <MAGNET_URI> <MAGNET_URI2> ...

Usage:
  p2pfile [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  download    Download file from magnet uri.
  help        Help about any command
  serve       creates and seeds a torrent from filepaths.

Flags:
      --config string          config file (default is $HOME/.p2pfile.yaml)
      --ip string              Set ip. (default: default route ip)
      --port int               Set port. (default: random port in port-range,  See --port-range)
      --port-range string      Set random port range. (default: 42070-42099) (default "42070-42099")
      --peers strings          Set bootstrap peers. (default: empty) (eg: --peers 10.1.1.1:2233,10.2.2.2:4567
      --upload-limit float     Set upload limit, MiB. (default: 0.0)
      --download-limit float   Set download limit, MiB. (default: 0.0)
      --debug                  Debug mode.
  -h, --help                   help for p2pfile

Use "p2pfile [command] --help" for more information about a command.
```

## 其他设计

A. Magnet URI schema（使用了[BEP0009](http://www.bittorrent.org/beps/bep_0009.html) 扩展）

`magnet:?xt=urn:btih:<info-hash>&dn=<name>&tr=<tracker>&x.pe=<peer-address>`

- `info-hash`: 哈希值，用于标识文件
- `name`: 文件名 [可选]
- `peer-address`: 做种机器的 Peer 地址，用于初始化 DHT 网络
- `tracker`: tracker 地址

B. Tracker 高可用性：

- 考虑到分发文件只是一次性命令，暂时不考虑

C. 下载后持续做种：

- 考虑到一次性命令，故不考虑

D. 任务中断恢复：

- 考虑到一次性命令，故不考虑，下载失败后需要重新下载

## 后续计划

1. resume download
2. tracker ha
3. multi file
4. download and upload speed limit
5. support set download directory

## 参考资料

- <https://github.com/anacrolix/torrent>: 第一版参考，因为下载速度较慢，放弃
- <https://github.com/cenkalti/rain>: 主要引用
- <https://gitlab.com/axet/libtorrent>: 后续参考实现 14: Local Peers Discovery / 19: WebSeeds.
