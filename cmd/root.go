package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "p2pfile",
	Short: "Simple P2P file distribution CLI",
	Long: `Simple P2P file distribution CLI. For example:

p2pfile serve <FILE_PATH>
p2pfile download <MAGNET_URI>`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().SortFlags = false

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.p2pfile.yaml)")
	rootCmd.PersistentFlags().Float64("download-limit", 0.0, "Set download limit, MiB. (default: 0.0)")
	rootCmd.PersistentFlags().Float64("upload-limit", 0.0, "Set upload limit, MiB. (default: 0.0)")
	rootCmd.PersistentFlags().Bool("debug", false, "Debug mode.")

	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("upload-limit", rootCmd.PersistentFlags().Lookup("upload-limit"))
	viper.BindPFlag("download-limit", rootCmd.PersistentFlags().Lookup("download-limit"))

	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newDownloadCmd())
	rootCmd.AddCommand(newVersionCmd())
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".p2pfile")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func initLogger(debug bool) {
	// Log as JSON instead of the default ASCII formatter.
	// log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	if debug {
		log.Infof("Logging level set to debug.")
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}
