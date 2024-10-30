package main

import (
	"flag"
	"fmt"
	"log"
	"nightlight/internal/service"
	"os"
	"os/signal"
	"path"

	"sync"
)

const pidFilename = "nightlight.pid"

func createPidFile() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	f, err := os.Create(path.Join(cwd, pidFilename))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("%d", os.Getpid()))
	if err != nil {
		return err
	}

	return nil
}

func deletePidFile() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.Remove(path.Join(cwd, pidFilename)); err != nil {
		return err
	}

	return nil
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/usr/local"
	} else {
		homeDir = path.Join(homeDir, ".config")
	}

	cfgFileArg := flag.String("config", path.Join(homeDir, "nightlight.toml"), "--config=[path to config file]")

	flag.Parse()

	if *cfgFileArg == "" {
		log.Fatal("No config file specified")
	}

	cfg, err := service.FromFile(*cfgFileArg)
	if err != nil {
		log.Fatal(err)
	}

	if !cfg.ValidateConfig() {
		log.Fatal("Invalid config")
	}

	stopCh := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	log.Println("Starting the nightlight service")
	var wg sync.WaitGroup
	wg.Add(1)
	go service.RunService(*cfgFileArg, stopCh, &wg)

	if err := createPidFile(); err != nil {
		stopCh <- struct{}{}
		log.Fatal("Failed to create PID file")
	}

	// wait to get a signal
	<-sig
	log.Println("Stopping the nightlight service")
	stopCh <- struct{}{}
	wg.Wait()
	close(stopCh)
	close(sig)

	if err := deletePidFile(); err != nil {
		log.Fatal("Failed to delete PID file")
	}
}
