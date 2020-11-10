package main

import (
	"log"

	"github.com/rancher/harvester-installer/pkg/console"
)

func main() {
	if err := console.RunConsole(); err != nil {
		log.Panicln(err)
	}
}
