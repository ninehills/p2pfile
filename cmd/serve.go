package cmd

import (
	"fmt"

	"github.com/ninehills/p2pfile/pkg/libtorrent"
	"github.com/ninehills/p2pfile/pkg/libtracker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newServeCmd() *cobra.Command {
	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "creates and seeds a torrent from filepaths.",
		Long: `Simple P2P file distribution CLI. Usage:

p2pfile serve <FILE_PATH1> <FILE_PATH2> ...`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: 写入这里才生效，需要改进
			initLogger(viper.GetBool("debug"))
			file := args[0]

			// 1. Start tracker
			trackerIP, err := libtorrent.GetPublicIP(viper.GetString("tracker-ip"))
			if err != nil {
				log.Fatal("Failed to get public ip", err)
			}
			trackerPort := viper.GetInt("tracker-port")
			if trackerPort == 0 {
				// TODO: 使用随机端口会导致 serve 服务重启后原 magnet uri 失效
				trackerPort, err = libtorrent.GetAvailablePort(viper.GetString("tracker-port-range"))
				if err != nil {
					log.Fatalf("Couldn't get available port: %v", err)
				} else {
					log.Infof("Founded available port: %v", trackerPort)
				}
			}
			log.Infof("Start tracker: http://%s:%d/1/announce ", trackerIP, trackerPort)
			go libtracker.RunTrackerServer(fmt.Sprintf(":%d", trackerPort))

			// 2. make torrent
			torrentFile := file + ".torrent"
			files := []string{file}
			trackers := []string{fmt.Sprintf("http://%s:%d/1/announce", trackerIP, trackerPort)}
			log.Infof("Make torrent %s to %s", file, torrentFile)
			magnet, err := libtorrent.CreateTorrent(files, torrentFile, "", "", false, 0, "", trackers, []string{})
			if err != nil {
				log.Fatal("Failed to create torrent: ", err)
			}
			log.Infof("Magnet: %s", magnet)
			// 3. Start torrent uploader
			err = libtorrent.RunTorrentServer(torrentFile, viper.GetString("dir"), true, false)
			if err != nil {
				log.Fatal("Failed to run torrent server: ", err)
			}

		},
	}
	return serveCmd
}
