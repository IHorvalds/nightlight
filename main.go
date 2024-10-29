package main

import (
	"flag"
	"log"

	"github.com/IHorvalds/nightlight/internal/service"
)

func main() {
	cfgFileArg := flag.String("config", "$HOME/.config/nightlight.toml", "--config=[path to config file]")
	installArg := flag.Bool("install", false, "Install the systemd service")

	flag.Parse()

	if *cfgFileArg == "" {
		log.Fatal("No config file specified")
	}

	cfg, err := service.FromFile(*cfgFileArg)

	if *installArg {
		// Install the service
		// and exit
	}

	// By default, run the service
	// create a pid file to ensure only one instance is running
}
