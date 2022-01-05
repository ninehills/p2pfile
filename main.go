package main

import (
	"github.com/ninehills/p2pfile/cmd"
)

var Version string

func main() {
	cmd.Version = Version
	cmd.Execute()
}
