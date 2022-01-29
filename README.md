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

做种：

```txt
Creates and seeds a torrent from file paths. Usage:

p2pfile serve <FILE_PATH>

Usage:
  p2pfile serve [flags]

Flags:
      --tracker-ip string           Set tracker ip. (default: default route ip)
      --tracker-port int            Set tracker port. (default: random port in port-range,  See --port-range)
      --tracker-port-range string   Set tracker random port range. (default: 42070-42099) (default "42070-42099")
  -h, --help                        help for serve

Global Flags:
      --config string   config file (default is $HOME/.p2pfile.yaml)
      --debug           Debug mode.
      --download-limit float   Set download limit, MiB. (default: 0.0)
      --upload-limit float     Set upload limit, MiB. (default: 0.0)
```

下载：

```txt
Download file from magnet uri. Usage:

p2pfile download <MAGNET_URI>

Usage:
  p2pfile download [flags]

Flags:
      --seeding                Seeding after download
      --seeding-max-time int   Seeding after download finish max time in seconds. default: 600(10min) (default 600)
      --seeding-auto-stop      Stop seeding after all nodes download finish. default: true (default true)
      --dir string             Set download dir. (default: .)
  -h, --help                   help for download

Global Flags:
      --config string   config file (default is $HOME/.p2pfile.yaml)
      --debug           Debug mode.
      --download-limit float   Set download limit, MiB. (default: 0.0)
      --upload-limit float     Set upload limit, MiB. (default: 0.0)
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

## 参考资料

- <https://github.com/anacrolix/torrent>: 第一版参考，因为下载速度较慢，放弃
- <https://github.com/cenkalti/rain>: 主要引用
- <https://gitlab.com/axet/libtorrent>: 后续参考实现 14: Local Peers Discovery / 19: WebSeeds.
