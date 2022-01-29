// Lots of code is copied from project [rain](https://github.com/cenkalti/rain/blob/master/main.go).
// Project torrent is under MIT license, and is compatible with GPLv3.

package libtorrent

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/rain/torrent"
	"github.com/ninehills/p2pfile/pkg/magnet"
	"github.com/ninehills/p2pfile/pkg/metainfo"
	log "github.com/sirupsen/logrus"
)

type TorrentServer struct {
	Target string
	// 下载路径
	DataDir string
	// 是否为做种节点，如是则调大连接数等参数。
	IsServe bool
	// 是否通过 *.resume 文件恢复下载
	IsResume bool
	// 下载完成做种的最大时间，超过后做种停止。单位为秒。默认为 0s。
	//		- 如 isServe = true，那么 maxSeedingSeconds 如等于 0，则代表永不停止（此逻辑上层暂时没有使用）
	//		- 如 isServe = false，那么 maxSeedingSeconds 如等于 0，则代表不做种
	MaxSeedingSeconds int
	// 当注册到 Tracker 的所有节点下载完成后，是否自动停止做种。（和 isServe 互斥）
	SeedingAutoStop bool
	// Global download speed limit in MB/s.
	SpeedLimitDownload float64
	// Global upload speed limit in MB/s.
	SpeedLimitUpload float64
}

func (s *TorrentServer) Run() error {
	log.Infof("Starting torrent server with config: %+v", s)
	cfg := torrent.DefaultConfig
	cfg.RPCEnabled = false
	cfg.DHTEnabled = false
	cfg.DataDir = s.DataDir
	cfg.DataDirIncludesTorrentID = false
	cfg.SpeedLimitDownload = int64(s.SpeedLimitDownload * 1024)
	cfg.SpeedLimitUpload = int64(s.SpeedLimitUpload * 1024)

	if s.IsServe {
		// 做种的节点调大连接数等配置
		cfg.UnchokedPeers = 100
		cfg.OptimisticUnchokedPeers = 10
		cfg.MaxRequestsIn = 2000
		cfg.MaxRequestsOut = 2000
		cfg.DefaultRequestsOut = 1000
		cfg.EndgameMaxDuplicateDownloads = 20
		cfg.MaxPeerDial = 1000
		cfg.MaxPeerAccept = 500
		cfg.MaxPeerAddresses = 20000

		if s.SeedingAutoStop {
			return fmt.Errorf("seedingAutoStop can't be true when isServe is true")
		}
	}
	enableSeeding := s.IsServe || s.MaxSeedingSeconds > 0

	var ih torrent.InfoHash
	var resumeFileName string
	if isURI(s.Target) {
		magnet, err := magnet.New(s.Target)
		if err != nil {
			return err
		}
		ih = torrent.InfoHash(magnet.InfoHash)
		resumeFileName = magnet.Name + ".resume"
	} else {
		f, err := os.Open(s.Target)
		if err != nil {
			return err
		}
		defer f.Close()
		mi, err := metainfo.New(f)
		if err != nil {
			return err
		}
		_ = f.Close()
		ih = mi.Info.Hash
		resumeFileName = mi.Info.Name + ".resume"
	}
	resumeFile := path.Join(s.DataDir, resumeFileName)
	log.Infof("Download resume file: %s, it will be auto delete when download finished.", resumeFile)

	if !s.IsResume {
		if _, err := os.Stat(resumeFile); !os.IsNotExist(err) {
			log.Infof("Not enable resume, so remove resume file: %s", resumeFile)
			if err := os.Remove(resumeFile); err != nil {
				return err
			}
		}
	}
	cfg.Database = resumeFile

	log.Debugf("Torrent new session with config %+v", cfg)
	ses, err := torrent.NewSession(cfg)
	if err != nil {
		return err
	}
	defer ses.Close()
	var t *torrent.Torrent
	torrents := ses.ListTorrents()
	if len(torrents) > 0 && torrents[0].InfoHash() == ih {
		// Resume data exists
		t = torrents[0]
		err = t.Start()
	} else {
		// Add as new torrent
		opt := &torrent.AddTorrentOptions{
			StopAfterDownload: !enableSeeding,
		}
		if isURI(s.Target) {
			t, err = ses.AddURI(s.Target, opt)
		} else {
			var f *os.File
			f, err = os.Open(s.Target)
			if err != nil {
				return err
			}
			t, err = ses.AddTorrent(f, opt)
			f.Close()
		}
	}
	if err != nil {
		return err
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case s := <-ch:
			log.Infof("received %s, stopping server", s)
			err = t.Stop()
			if err != nil {
				return err
			}
		case <-time.After(time.Second):
			stats := t.Stats()
			progress := 0
			if stats.Bytes.Total > 0 {
				progress = int((stats.Bytes.Completed * 100) / stats.Bytes.Total)
			}
			eta := "?"
			if stats.ETA != nil {
				eta = stats.ETA.String()
			}
			log.Infof(
				"Status: %s, Progress: %d%%, Peers: %d(%din/%dout), Download: %dK/s, Upload: %dK/s, ETA: %s, Seeding: %s\n",
				stats.Status.String(), progress, stats.Peers.Total, stats.Peers.Incoming, stats.Peers.Outgoing,
				stats.Speed.Download/1024, stats.Speed.Upload/1024, eta, stats.SeededFor.Truncate(time.Second).String(),
			)
			// 如果 maxSeedingSeconds 大于 0，则控制做种不能超过此值，对 isServe = true/false 均有效
			if s.MaxSeedingSeconds > 0 && stats.Status == torrent.Seeding && stats.SeededFor.Seconds() > float64(s.MaxSeedingSeconds) {
				log.Infof("Seeding max time %d is reached, stop seeding.", s.MaxSeedingSeconds)
				err = t.Stop()
				if err != nil {
					log.Errorf("Stop seeding error: %s", err)
					return err
				}
			}
			// 如果开启了 seedingAutoStop，那么就检查 Tracker 中是否还有未完成的节点，如果有则停止
			// Tracker 默认的最小检查周期是 1min，所以此处存在一定的延迟，所以除非文件很大或单节点的带宽较小，否则不建议开启下载后做种功能
			if s.SeedingAutoStop && stats.Status == torrent.Seeding {
				willStop := true
				for _, tracker := range t.Trackers() {
					log.Debugf(
						"tracker: %s status:%d leechers:%d seeders: %d LastAnnounce:%s",
						tracker.URL, tracker.Status, tracker.Leechers, tracker.Seeders, tracker.LastAnnounce.String())
					if tracker.Leechers > 0 {
						willStop = false
						break
					}
				}
				if willStop {
					log.Infof("All tracker has no leechers, stop seeding.")
					err = t.Stop()
					if err != nil {
						log.Errorf("Stop seeding error: %s", err)
						return err
					}
				}
			}
		case err = <-t.NotifyStop():
			if err != nil {
				log.Warnf("Torrent stopped: %s", err)
				return err
			}
			// TODO: 只有下载完成才应该删除 resumeFile
			log.Infof("Torrent stopped normally, so remove resume file")
			if _, err := os.Stat(resumeFile); !os.IsNotExist(err) {
				os.Remove(resumeFile)
			}
			return nil
		}
	}
}

// @param files: include this file or directory in torrent
// @param out: save generated torrent to this `FILE`
// @param root: file paths given become relative to the root
// @param name: set name of torrent. required if you specify more than one file.
// @param private: create torrent for private trackers
// @param pieceLength: override default piece length. by default, piece length calculated automatically based on the total size of files. given in KB. must be multiple of 16.
// @param comment: set comment of torrent
// @param trackers: add tracker `URL`
// @param webseeds: add web seed `URL`
func CreateTorrent(files []string, out string, root string, name string, private bool, pieceLength int, comment string, trackers []string, webseeds []string) (string, error) {
	var err error
	tiers := make([][]string, len(trackers))
	for i, tr := range trackers {
		tiers[i] = []string{tr}
	}

	info, err := metainfo.NewInfoBytes(root, files, private, uint32(pieceLength<<10), name)
	if err != nil {
		return "", err
	}
	mi, err := metainfo.NewBytes(info, tiers, webseeds, comment)
	if err != nil {
		return "", err
	}
	log.Infof("Created torrent size: %d bytes", len(mi))
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	_, err = f.Write(mi)
	if err != nil {
		return "", err
	}

	i, err := metainfo.NewInfo(info)
	if err != nil {
		return "", err
	}

	magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", string(i.HashString()), i.Name)
	trackersEscaped := make([]string, len(trackers))
	for _, s := range trackers {
		trackersEscaped = append(trackersEscaped, url.QueryEscape(s))
	}
	if len(trackersEscaped) > 0 {
		magnet += strings.Join(trackersEscaped, "&tr=")
	}
	return magnet, f.Close()
}
