package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"nightlight/internal/service"
	"os"
	"os/exec"
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

func run(cfgFile string) error {
	cfg, err := service.FromFile(cfgFile)
	if err != nil {
		return err
	}

	if !cfg.ValidateConfig() {
		return errors.New("invalid configuration")
	}

	stopCh := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	log.Println("Starting the nightlight service")
	var wg sync.WaitGroup
	wg.Add(1)
	go service.RunService(cfgFile, stopCh, &wg)

	if err := createPidFile(); err != nil {
		stopCh <- struct{}{}
		return errors.New("failed to create PID file")
	}

	// wait to get a signal
	<-sig
	log.Println("Stopping the nightlight service")
	stopCh <- struct{}{}
	wg.Wait()
	close(stopCh)
	close(sig)

	if err := deletePidFile(); err != nil {
		return fmt.Errorf("failed to delete PID file: %w", err)
	}

	return nil
}

func startServiceProcess(cfg string) error {
	var procAttr os.ProcAttr

	prog := os.Args[0]
	if !path.IsAbs(prog) {
		_prog, err := exec.LookPath(path.Base(prog))
		if err != nil {
			return err
		}

		prog = _prog
	}

	procAttr.Files = []*os.File{os.Stdin,
		os.Stdout, os.Stderr}
	procAttr.Env = os.Environ()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	procAttr.Dir = cwd
	_, err = os.StartProcess(prog, []string{"--config", cfg, "--svc"}, &procAttr)
	return err
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/usr/local"
	} else {
		homeDir = path.Join(homeDir, ".config")
	}

	cfgFileArg := flag.String("config", path.Join(homeDir, "nightlight.toml"), "--config=[path to config file]")
	svcArg := flag.Bool("svc", false, "--svc")

	flag.Parse()

	if *cfgFileArg == "" {
		log.Fatal("No config file specified")
	}

	if *svcArg {
		if err := run(*cfgFileArg); err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Starting nightlight service process")
	if err := startServiceProcess(*cfgFileArg); err != nil {
		log.Fatal(err)
	}

}
