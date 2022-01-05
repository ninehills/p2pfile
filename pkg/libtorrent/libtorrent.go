// Lots of code is copied from project [torrent](https://github.com/anacrolix/torrent).
// Project torrent is under MPL 2.0 license, and is compatible with GPLv3.

package libtorrent

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/dht/v2"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

func torrentBar(t *torrent.Torrent, pieceStates bool) {
	go func() {
		start := time.Now()
		if t.Info() == nil {
			fmt.Printf("%v: getting torrent info for %q\n", time.Since(start), t.Name())
			<-t.GotInfo()
		}
		lastStats := t.Stats()
		var lastLine string
		interval := 3 * time.Second
		for range time.Tick(interval) {
			var completedPieces, partialPieces int
			psrs := t.PieceStateRuns()
			for _, r := range psrs {
				if r.Complete {
					completedPieces += r.Length
				}
				if r.Partial {
					partialPieces += r.Length
				}
			}
			stats := t.Stats()
			byteRate := int64(time.Second)
			byteRate *= stats.BytesReadUsefulData.Int64() - lastStats.BytesReadUsefulData.Int64()
			byteRate /= int64(interval)
			line := fmt.Sprintf(
				"%v: downloading %q: %s/%s, %d/%d pieces completed (%d partial): %v/s\n",
				time.Since(start),
				t.Name(),
				humanize.Bytes(uint64(t.BytesCompleted())),
				humanize.Bytes(uint64(t.Length())),
				completedPieces,
				t.NumPieces(),
				partialPieces,
				humanize.Bytes(uint64(byteRate)),
			)
			if line != lastLine {
				lastLine = line
				os.Stdout.WriteString(line)
			}
			if pieceStates {
				fmt.Println(psrs)
			}
			lastStats = stats
		}
	}()
}

type stringAddr string

func (stringAddr) Network() string   { return "" }
func (me stringAddr) String() string { return string(me) }

func resolveTestPeers(addrs []string) (ret []torrent.PeerInfo) {
	for _, ta := range addrs {
		ret = append(ret, torrent.PeerInfo{
			Addr: stringAddr(ta),
		})
	}
	return
}

type TorrentServer struct {
	clientConfig *torrent.ClientConfig
	client       *torrent.Client

	peers     []string
	torrents  []string
	files     []string
	localAddr string
}

func NewTorrentServer(mode string, ip string, port int, portRange string, peers []string, uploadLimit float64, downloadLimit float64, debug bool, torrents []string, files []string) (TorrentServer, error) {
	var err error
	log.Infof("NewTorrentServer: %s, %s, %d, %v, %f, %f, %t, %v, %v", mode, ip, port, peers, uploadLimit, downloadLimit, debug, torrents, files)
	server := TorrentServer{}
	clientConfig := torrent.NewDefaultClientConfig()
	clientConfig.DisableIPv6 = false
	clientConfig.DisableAcceptRateLimiting = true
	clientConfig.DisableTrackers = true
	clientConfig.NoDHT = false
	clientConfig.Debug = debug
	if mode == "download" {
		// TODO: download mode support seeding.
		clientConfig.Seed = false
	} else {
		clientConfig.Seed = true
	}

	var publicIp net.IP
	if ip != "" {
		publicIp = net.ParseIP(ip)
	} else {
		publicIp, err = getPublicIP()
		if err != nil {
			log.Fatalf("failed to get default public ip: %s")
		} else {
			log.Infof("get default public ip: %s", publicIp)
		}
	}

	if port == 0 {
		// TODO: 使用随机端口会导致 serve 服务重启后原 magnet uri 失效
		port, err = GetAvailablePort(portRange)
		if err != nil {
			log.Fatalf("Couldn't get available port: %v", err)
		} else {
			log.Infof("Founded available port: %v", port)
		}
	}
	clientConfig.PublicIp4 = publicIp
	clientConfig.ListenPort = port
	localAddr := fmt.Sprintf("%s:%d", publicIp, port)
	clientConfig.SetListenAddr(fmt.Sprintf(":%d", port))
	// 不使用 Public 的 DHT Starting Nodes，如果配置了 peer，则使用 peer，否则留空。
	// 问题是如果没有 peer，就组成不了 DHT 网络，有大量的错误信息。
	// 解决办法就是将自身的 IP 加入到 Staring Nodes 中，这样就可以组成 1 节点的 DHT 网络。
	clientConfig.DhtStartingNodes = func(network string) dht.StartingNodesGetter {
		return func() ([]dht.Addr, error) {
			var addrs []dht.Addr
			for _, p := range peers {
				addr, err := net.ResolveUDPAddr("udp", p)
				if err != nil {
					return addrs, fmt.Errorf("peer %v not resolved: %v", p, err)
				}
				addrs = append(addrs, dht.NewAddr(addr))
			}
			addr, err := net.ResolveUDPAddr("udp", localAddr)
			if err != nil {
				return addrs, fmt.Errorf("self addr %v not resolved: %v", addr, err)
			}
			addrs = append(addrs, dht.NewAddr(addr))
			return addrs, nil
		}
	}

	if uploadLimit > 0 {
		log.Printf("Upload speed limit: %v MiB/s", uploadLimit)
		clientConfig.UploadRateLimiter = rate.NewLimiter(rate.Limit(uploadLimit*1024*1024), 256<<10)
	}

	if downloadLimit > 0 {
		log.Printf("Download speed limit: %v MiB/s", downloadLimit)
		clientConfig.DownloadRateLimiter = rate.NewLimiter(rate.Limit(downloadLimit*1024*1024), 256<<10)
	}

	client, err := torrent.NewClient(clientConfig)

	if err != nil {
		return server, fmt.Errorf("creating client: %w", err)
	}

	server.client = client
	server.clientConfig = clientConfig
	server.peers = peers
	server.torrents = torrents
	server.files = files
	server.localAddr = localAddr
	return server, nil
}

func (td *TorrentServer) AddTorrent() error {
	peers := resolveTestPeers(td.peers)
	for _, arg := range td.torrents {
		t, err := func() (*torrent.Torrent, error) {
			if strings.HasPrefix(arg, "magnet:") {
				t, err := td.client.AddMagnet(arg)
				if err != nil {
					return nil, fmt.Errorf("error adding magnet: %w", err)
				}
				return t, nil
			} else if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				response, err := http.Get(arg)
				if err != nil {
					return nil, fmt.Errorf("error downloading torrent file: %s", err)
				}

				metaInfo, err := metainfo.Load(response.Body)
				defer response.Body.Close()
				if err != nil {
					return nil, fmt.Errorf("error loading torrent file %q: %s", arg, err)
				}
				t, err := td.client.AddTorrent(metaInfo)
				if err != nil {
					return nil, fmt.Errorf("adding torrent: %w", err)
				}
				return t, nil
			} else if strings.HasPrefix(arg, "infohash:") {
				t, _ := td.client.AddTorrentInfoHash(metainfo.NewHashFromHex(strings.TrimPrefix(arg, "infohash:")))
				return t, nil
			} else {
				metaInfo, err := metainfo.LoadFromFile(arg)
				if err != nil {
					return nil, fmt.Errorf("error loading torrent file %q: %s", arg, err)
				}
				t, err := td.client.AddTorrent(metaInfo)
				if err != nil {
					return nil, fmt.Errorf("adding torrent: %w", err)
				}
				return t, nil
			}
		}()
		if err != nil {
			return fmt.Errorf("adding torrent for %q: %w", arg, err)
		}

		torrentBar(t, false)
		log.Printf("added peer: %v", peers)
		t.AddPeers(peers)
		go func() {
			<-t.GotInfo()
			if len(td.files) == 0 {
				t.DownloadAll()
			} else {
				for _, f := range t.Files() {
					for _, fileArg := range td.files {
						if f.DisplayPath() == fileArg {
							f.Download()
						}
					}
				}
			}
		}()
	}
	return nil
}

func (td *TorrentServer) outputStats() {
	expvar.Do(func(kv expvar.KeyValue) {
		fmt.Printf("%s: %s\n", kv.Key, kv.Value)
	})
	td.client.WriteStatus(os.Stdout)
}

func (td *TorrentServer) RunDownloader(ctx context.Context) error {
	err := td.AddTorrent()
	started := time.Now()
	if err != nil {
		return fmt.Errorf("adding torrents error: %w", err)
	}
	// defer td.outputStats()
	go func() {
		<-ctx.Done()
		td.Shutdown()
	}()
	if td.client.WaitAll() {
		log.Print("downloaded ALL the torrents")
	} else {
		err = errors.New("some torrents were not downloaded successfully")
	}
	clientConnStats := td.client.ConnStats()
	log.Printf("average download rate: %v",
		humanize.Bytes(uint64(
			time.Duration(
				clientConnStats.BytesReadUsefulData.Int64(),
			)*time.Second/time.Since(started),
		)))

	spew.Dump(expvar.Get("torrent").(*expvar.Map).Get("chunks received"))
	spew.Dump(td.client.ConnStats())
	clStats := td.client.ConnStats()
	sentOverhead := clStats.BytesWritten.Int64() - clStats.BytesWrittenData.Int64()
	log.Printf(
		"client read %v, %.1f%% was useful data. sent %v non-data bytes",
		humanize.Bytes(uint64(clStats.BytesRead.Int64())),
		100*float64(clStats.BytesReadUsefulData.Int64())/float64(clStats.BytesRead.Int64()),
		humanize.Bytes(uint64(sentOverhead)))
	td.Shutdown()
	return err
}

func (td *TorrentServer) Shutdown() {
	log.Infof("shutting down torrent server...")
	// In certain situations, close was being called more than once.
	select {
	case <-td.client.Closed():
	default:
		td.client.Close()
	}
	log.Infof("torrent server shut down")
}

func (td *TorrentServer) RunServer(ctx context.Context) error {
	var err error
	peers := append(td.peers, td.localAddr)
	peersStr := strings.Join(peers[:], ",")
	for _, file := range td.files {
		info := metainfo.Info{
			PieceLength: 1 << 18,
		}
		err = info.BuildFromFilePath(file)
		if err != nil {
			return fmt.Errorf("building info from path %q: %w", file, err)
		}
		mi := metainfo.MetaInfo{
			InfoBytes: bencode.MustMarshal(info),
		}
		pc, err := storage.NewDefaultPieceCompletionForDir(".")
		if err != nil {
			return fmt.Errorf("new piece completion: %w", err)
		}
		defer pc.Close()
		ih := mi.HashInfoBytes()
		to, _ := td.client.AddTorrentOpt(torrent.AddTorrentOpts{
			InfoHash: ih,
			Storage: storage.NewFileOpts(storage.NewFileClientOpts{
				ClientBaseDir: file,
				FilePathMaker: func(opts storage.FilePathMakerOpts) string {
					return filepath.Join(opts.File.Path...)
				},
				TorrentDirMaker: nil,
				PieceCompletion: pc,
			}),
		})
		defer to.Drop()
		err = to.MergeSpec(&torrent.TorrentSpec{
			InfoBytes: mi.InfoBytes,
			Trackers:  [][]string{{}},
		})
		if err != nil {
			return fmt.Errorf("setting trackers: %w", err)
		}
		fmt.Printf("file: %q\n", file)
		fmt.Printf("uri: magnet:?xt=urn:btih:%s&dn=name&x.pe=%s\n", ih, peersStr)
	}
	<-ctx.Done()
	// FIXME: sever shutdown call client.Close(), it's panic.
	// td.Shutdown()
	return ctx.Err()
}
