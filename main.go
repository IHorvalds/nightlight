package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"nightlight/internal/service"
	"os"
	"os/exec"
	"path"
)

func startServiceProcess(cfg string) error {
	prog := os.Args[0]
	if !path.IsAbs(prog) {
		_prog, err := exec.LookPath(path.Base(prog))
		if err != nil {
			return err
		}

		prog = _prog
	}

	c := exec.Command(prog, "-config", cfg, "-svc")
	defer func() {
		if c.Process != nil {
			c.Process.Release()
		}
	}()

	return c.Start()
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	cfgFileArg := flag.String("config", path.Join(homeDir, ".config", "nightlight", "nightlight.toml"), "-config=[path to config file]")
	svcArg := flag.Bool("svc", false, "Run as a service")
	stopSvcArg := flag.Bool("stop", false, "Stops the service")

	flag.Parse()

	if *stopSvcArg {
		if err := service.StopService(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	if *cfgFileArg == "" {
		log.Fatal("No config file specified")
	}

	err = os.MkdirAll(path.Dir(*cfgFileArg), 0744)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(*cfgFileArg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, err := os.Create(*cfgFileArg)
			if err != nil {
				log.Fatal(err)
			}
			f.Write([]byte(service.EmptyConfig))
			f.Close()
			log.Fatalf("Empty config file at %s", *cfgFileArg)
		} else {
			log.Fatal(err)
		}
	}

	if *svcArg {
		if err := service.RunService(*cfgFileArg); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	cfg, err := service.FromFile(*cfgFileArg)
	if err != nil {
		log.Fatal(fmt.Errorf("invalid config file (%s): %w", *cfgFileArg, err))
	}

	if err = cfg.ValidateConfig(); err != nil {
		log.Fatal(fmt.Errorf("invalid config file (%s): %w", *cfgFileArg, err))
	}

	if err := startServiceProcess(*cfgFileArg); err != nil {
		log.Fatal(err)
	}

}
