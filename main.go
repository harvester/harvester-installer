package main

import (
	"context"
	"log"
	"os"

	"github.com/harvester/harvester-installer/pkg/console"
	"github.com/harvester/harvester-installer/pkg/version"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:    "harvester-installer",
		Version: version.FriendlyVersion(),
		Usage:   "Console application to install Harvester",
		UsageText: `harvester-installer [global options] [command [command options]]

Executes the Harvester installer if no command is specified.`,
		Action: func(context.Context, *cli.Command) error {
			return console.RunConsole()
		},
		Commands: []*cli.Command{},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
