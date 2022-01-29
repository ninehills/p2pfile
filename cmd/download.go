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

			seedingMaxTime := 0
			// 当开启 seeding 后，seeding-max-time 才会生效
			if viper.GetBool("seeding") {
				seedingMaxTime = viper.GetInt("seeding-max-time")
			}
			err := libtorrent.RunTorrentServer(
				args[0], viper.GetString("dir"), false, false,
				seedingMaxTime, viper.GetBool("seeding-auto-stop"),
			)
			if err != nil {
				log.Fatal("Failed to run torrent server: ", err)
			}
		},
	}
	downloadCmd.Flags().SortFlags = false
	downloadCmd.Flags().Bool("seeding", false, "Seeding after download")
	downloadCmd.Flags().Int("seeding-max-time", 600, "Seeding after download finish max time in seconds. default: 600(10min)")
	downloadCmd.Flags().Bool("seeding-auto-stop", true, "Stop seeding after all nodes download finish. default: true")
	downloadCmd.Flags().String("dir", "", "Set download dir. (default: .)")

	viper.BindPFlag("dir", downloadCmd.Flags().Lookup("dir"))
	viper.BindPFlag("seeding", downloadCmd.Flags().Lookup("seeding"))
	viper.BindPFlag("seeding-max-time", downloadCmd.Flags().Lookup("seeding-max-time"))
	viper.BindPFlag("seeding-auto-stop", downloadCmd.Flags().Lookup("seeding-auto-stop"))
	return downloadCmd
}
