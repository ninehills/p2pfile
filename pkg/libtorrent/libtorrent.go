// Lots of code is copied from project [rain](https://github.com/cenkalti/rain/blob/master/main.go).
// Project torrent is under MIT license, and is compatible with GPLv3.

package libtorrent

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/rain/torrent"
	"github.com/ninehills/p2pfile/pkg/magnet"
	"github.com/ninehills/p2pfile/pkg/metainfo"
	log "github.com/sirupsen/logrus"
)

func RunTorrentServer(target string, dataDir string, isSeed bool, isResume bool) error {
	cfg := torrent.DefaultConfig
	cfg.RPCEnabled = false
	cfg.DHTEnabled = false
	cfg.DataDir = dataDir
	cfg.DataDirIncludesTorrentID = false

	if isSeed {
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
	}

	var ih torrent.InfoHash
	var resumeFile string
	if isURI(target) {
		magnet, err := magnet.New(target)
		if err != nil {
			return err
		}
		ih = torrent.InfoHash(magnet.InfoHash)
		resumeFile = magnet.Name + ".resume"
	} else {
		f, err := os.Open(target)
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
		resumeFile = mi.Info.Name + ".resume"
	}
	if !isResume {
		if _, err := os.Stat(resumeFile); !os.IsNotExist(err) {
			log.Infof("Not enable resume, so remove resume file: %s", resumeFile)
			if err := os.Remove(resumeFile); err != nil {
				return err
			}
		}
	}
	cfg.Database = resumeFile

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
			StopAfterDownload: !isSeed,
		}
		if isURI(target) {
			t, err = ses.AddURI(target, opt)
		} else {
			var f *os.File
			f, err = os.Open(target)
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
			log.Infof("Status: %s, Progress: %d%%, Peers: %d, Speed: %dK/s, ETA: %s\n", stats.Status.String(), progress, stats.Peers.Total, stats.Speed.Download/1024, eta)
		case err = <-t.NotifyStop():
			if _, err := os.Stat(resumeFile); !os.IsNotExist(err) {
				os.Remove(resumeFile)
			}
			return err
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
