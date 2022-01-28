package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ninehills/p2pfile/pkg/libtorrent"

	log "github.com/sirupsen/logrus"
)

func newDownloadCmd() *cobra.Command {
	var downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Download file from magnet uri.",
		Long: `Download file from magnet uri. Usage:

p2pfile download <MAGNET_URI>`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: 写入这里才生效，需要改进
			initLogger(viper.GetBool("debug"))

			err := libtorrent.RunTorrentServer(args[0], viper.GetString("dir"), false, false)
			if err != nil {
				log.Fatal("Failed to run torrent server: ", err)
			}
		},
	}
	return downloadCmd
}
