package main

import (
	"log"

	"github.com/harvester/harvester-installer/pkg/console"
)

func main() {
	if err := console.RunConsole(); err != nil {
		log.Panicln(err)
	}
}
