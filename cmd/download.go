package cmd

import (
	"context"
	"os"
	"os/signal"

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

p2pfile download <MAGNET_URI> <MAGNET_URI2> ...`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: 写入这里才生效，需要改进
			initLogger(viper.GetBool("debug"))
			// TODO: support set download directory
			downloader, err := libtorrent.NewTorrentServer(
				"download", viper.GetString("ip"), viper.GetInt("port"), viper.GetString("port-range"),
				viper.GetStringSlice("peers"), viper.GetFloat64("upload-limit"),
				viper.GetFloat64("download-limit"), viper.GetBool("debug"),
				args, []string{},
			)
			if err != nil {
				log.Fatal("Failed to create downloader: ", err)
			}
			ctx := context.Background()

			// trap Ctrl+C and call cancel on the context
			ctx, cancel := context.WithCancel(ctx)
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			defer func() {
				signal.Stop(c)
				cancel()
			}()
			go func() {
				select {
				case sig := <-c:
					log.Errorf("Caught signal %v, shutting down...\n", sig)
					cancel()
				case <-ctx.Done():
				}
			}()
			err = downloader.RunDownloader(ctx)
			if err != nil {
				log.Fatal("Failed to run downloader: ", err)
			}
		},
	}
	downloadCmd.MarkFlagRequired("magnet")
	return downloadCmd
}
