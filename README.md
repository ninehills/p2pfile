# p2pfile - DHT-based P2P file distribution command line tools

## 背景

### 应用场景

- 所有节点网络联通的环境下的文件分布式分发。
- 私有网络环境，和互联网隔离。
- 无文件加密传输需求

### 设计限制

- 不支持 Tracker，只支持 DHT 网络，从而简化设计。
- 不需要 Daemon 常驻进程，只需要单个二进制文件。
- 无加密设计
- 只支持单个文件分发，不支持文件夹分发。
- 不支持 IPv6。

### 设计目标

- 提供私有网络环境下的文件分布式分发。
- 提供最简化的使用方法，一条命令。

## 命令行设计

```txt
DHT-based P2P file distribution command line tools. For example:

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

`magnet:?xt=urn:btih:<info-hash>&dn=<name>&x.pe=<peer-address>`

- `info-hash`: 哈希值，用于标识文件
- `name`: 文件名[可选]
- `peer-address`: 做种机器的Peer地址，用于初始化 DHT 网络

一台机器并发做种或者下载：

- 由于无后台进程，每个进程都是单独的客户端，均需要占用一个端口。
- 故端口需要支持随机分配，从而避免端口冲突。
    - 随机分配方法：在端口段中随机选择端口，如果端口被占用，则重新随机选择。
- 随机分配存在当做种进程重启后，其新的端口号和之前不同。
    - 方法1：将端口号持久化，当重新serve的时候，使用之前的端口号
    - 方法2：重新做种的时候，需要传入magnet uri，从中解析出端口号。

B. 做种高可用性：

- 可以使用2+台机器同时做种，做种时增加参数：`--peers=<peer1>,<peer2>` 从而保证Magnet URI的高可用性。

C. 下载后持续做种：

- 增加参数 `--seeding`，指定下载之后持续做种。持续做种结束条件有多个，任意条件满足即停止做种。
    - `--seeding-time=<time>`，指定持续做种的时间，单位为秒。默认为 60s。
    - `--seeding-ratio=<ratio>`，指定持续做种的种子分享率，单位为百分比。默认为 1.0。
- 该参数推荐在大文件分发时使用，小文件分发没必要且不建议使用。

D. 信息存储：

- 默认为 SQlite，会在当前目录下生成 `.*db.*` 文件。
- (后续支持) 支持更换存储后端，比如 bolt/sqlite/file etc.

E. 库：

- <https://github.com/anacrolix/torrent>: 主要使用.
- <https://gitlab.com/axet/libtorrent>: 后续参考实现 14: Local Peers Discovery / 19: WebSeeds.
