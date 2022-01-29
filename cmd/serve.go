package cmd

import (
	"fmt"

	"github.com/ninehills/p2pfile/pkg/libtorrent"
	"github.com/ninehills/p2pfile/pkg/libtracker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Tracker Room Name
const trackerRoom = "1"

func newServeCmd() *cobra.Command {
	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "creates and seeds a torrent from file paths.",
		Long: `Creates and seeds a torrent from file paths. Usage:

p2pfile serve <FILE_PATH>`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: 写入这里才生效，需要改进
			debug := viper.GetBool("debug")
			initLogger(debug)
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
			trackerURL := fmt.Sprintf("http://%s:%d/%s/announce", trackerIP, trackerPort, trackerRoom)
			log.Infof("Start tracker: %s (debug: %v)", trackerURL, debug)
			go libtracker.RunTrackerServer(fmt.Sprintf(":%d", trackerPort), debug)

			// 2. make torrent
			torrentFile := file + ".torrent"
			files := []string{file}
			trackers := []string{trackerURL}
			log.Infof("Make torrent %s to %s", file, torrentFile)
			magnet, err := libtorrent.CreateTorrent(files, torrentFile, "", "", false, 0, "", trackers, []string{})
			if err != nil {
				log.Fatal("Failed to create torrent: ", err)
			}
			log.Infof("Magnet: %s", magnet)
			// 3. Start torrent uploader
			err = libtorrent.RunTorrentServer(torrentFile, viper.GetString("dir"), true, false, 0, false)
			if err != nil {
				log.Fatal("Failed to run torrent server: ", err)
			}
		},
	}
	serveCmd.Flags().SortFlags = false
	serveCmd.Flags().String("tracker-ip", "", "Set tracker ip. (default: default route ip)")
	serveCmd.Flags().Int("tracker-port", 0, "Set tracker port. (default: random port in port-range,  See --port-range)")
	serveCmd.Flags().String("tracker-port-range", "42070-42099", "Set tracker random port range. (default: 42070-42099)")

	viper.BindPFlag("tracker-ip", serveCmd.Flags().Lookup("tracker-ip"))
	viper.BindPFlag("tracker-port", serveCmd.Flags().Lookup("tracker-port"))
	viper.BindPFlag("tracker-port-range", serveCmd.Flags().Lookup("tracker-port-range"))
	return serveCmd
}
