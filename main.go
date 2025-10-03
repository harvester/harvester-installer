package main

import (
	"context"
	"errors"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/harvester/harvester-installer/pkg/config"
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

		Commands: []*cli.Command{
			{
				Name:  "generate-network-yaml",
				Usage: "Generate /oem YAML file for network configuration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Value: "/oem/harvester.config",
						Usage: "Harvester config file",
					},
					&cli.StringFlag{
						Name:  "cloud-init",
						Value: "/oem/91_networkmanager.yaml",
						Usage: "YAML file to generate",
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Overwrite YAML file if it already exists",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					data, err := os.ReadFile(cmd.String("config"))
					if err != nil {
						return err
					}
					harvesterCfg, err := config.LoadHarvesterConfig(data)
					if err != nil {
						return err
					}
					cosConfig, err := config.ConvertNetworkToCOS(harvesterCfg)
					if err != nil {
						return err
					}
					bytes, err := yaml.Marshal(cosConfig)
					if err != nil {
						return err
					}
					_, err = os.Stat(cmd.String("cloud-init"))
					if errors.Is(err, os.ErrNotExist) || cmd.Bool("force") {
						// Output file either doesn't exist, or we're forcing overwrite
						err = os.WriteFile(cmd.String("cloud-init"), bytes, 0600)
						if err != nil {
							return err
						}
						log.Printf("Generated %s from %s\n", cmd.String("cloud-init"), cmd.String("config"))
						return nil
					} else if err == nil {
						// File definitely exists (os.Stat was successful)
						log.Printf("Skipped generation of %s (file already exists, specify --force to overwrite)", cmd.String("cloud-init"))
						return nil
					} else {
						// File may or may not exists (some unexpected problem invoking os.Stat)
						return err
					}
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
