package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/ninehills/p2pfile/pkg/libtorrent"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newServeCmd() *cobra.Command {
	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "creates and seeds a torrent from filepaths.",
		Long: `DHT-based P2P file distribution command line tools. Usage:

p2pfile serve <FILE_PATH1> <FILE_PATH2> ...`,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: 写入这里才生效，需要改进
			initLogger(viper.GetBool("debug"))
			server, err := libtorrent.NewTorrentServer(
				"serve", viper.GetString("ip"), viper.GetInt("port"), viper.GetString("port-range"),
				viper.GetStringSlice("peers"), viper.GetFloat64("upload-limit"),
				viper.GetFloat64("download-limit"), viper.GetBool("debug"),
				[]string{}, args,
			)
			if err != nil {
				log.Fatal("Failed to create server: ", err)
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
			err = server.RunServer(ctx)
			if err != nil {
				log.Fatal("Failed to run server: ", err)
			}
		},
	}
	serveCmd.MarkFlagRequired("files")
	return serveCmd
}
