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
	"strconv"
	"syscall"

	"sync"

	"golang.org/x/sys/unix"
)

const (
	heartBeatMsg  = "hb"
	heartBeatResp = "al"
	pidFilename   = "nightlight.pid"
)

func createPidFile(pidFile string) error {
	f, err := os.Create(pidFile)
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

func otherInstanceExists(pidFile string) bool {
	_, err := os.Stat(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	if err != nil {
		return true
	}

	p, err := os.ReadFile(pidFile)
	if err != nil {
		return true
	}

	pid, err := strconv.ParseInt(string(p), 10, 32)
	if err != nil {
		return true
	}

	proc, _ := os.FindProcess(int(pid))
	defer proc.Release()

	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func stopService() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	pidFile := path.Join(cwd, pidFilename)
	p, err := os.ReadFile(pidFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	pid, err := strconv.ParseInt(string(p), 10, 32)
	if err != nil {
		return err
	}

	proc, _ := os.FindProcess(int(pid))
	defer proc.Release()

	return proc.Signal(os.Interrupt)
}

func run(cfgFile string) error {
	lockDir := "/var/lock"
	fi, err := os.Stat(lockDir)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return syscall.ENOTDIR
	}

	pidFile := path.Join(lockDir, pidFilename)
	if exists := otherInstanceExists(pidFile); exists {
		return fmt.Errorf("service already exists. Check %s", pidFile)
	}

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

	if err = createPidFile(pidFile); err != nil {
		stopCh <- struct{}{}
		return fmt.Errorf("failed to create PID file: %w", err)
	}

	// wait to get a signal
	<-sig
	log.Println("Stopping the nightlight service")
	stopCh <- struct{}{}
	wg.Wait()
	close(stopCh)
	close(sig)

	if err := os.Remove(pidFile); err != nil {
		return fmt.Errorf("failed to delete PID file: %w", err)
	}

	return nil
}

func startServiceProcess(cfg string) error {
	prog := os.Args[0]
	if !path.IsAbs(prog) {
		_prog, err := exec.LookPath(path.Base(prog))
		if err != nil {
			return err
		}

		prog = _prog
	}

	c := exec.Command(prog, "--config", cfg, "--svc")
	defer func() {
		if c.Process != nil {
			c.Process.Release()
		}
	}()

	return c.Start()
}

func isDirAndReadable(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return syscall.ENOTDIR
	}
	if err := unix.Access(dir, unix.R_OK); err != nil {
		return err
	}

	return nil
}

// Will return in order (first one that exists and is readable)
// - $HOME/.config/nightlight
// - /usr/local/share/nightlight
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/usr/local/share/nightlight"
		if err = isDirAndReadable(homeDir); err != nil {
			return "", err
		}

		return homeDir, nil
	}

	homeDir = path.Join(homeDir, ".config", "nightlight")
	if err = isDirAndReadable(homeDir); err != nil {
		return "", err
	}

	return homeDir, err
}

func main() {
	cfgDir, err := getConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	cfgFileArg := flag.String("config", path.Join(cfgDir, "nightlight.toml"), "-config=[path to config file]")
	svcArg := flag.Bool("svc", false, "Run as a service")
	stopSvcArg := flag.Bool("stop", false, "Stops the service")

	flag.Parse()

	if *stopSvcArg {
		if err := stopService(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	if *cfgFileArg == "" {
		log.Fatal("No config file specified")
	}

	if *svcArg {
		if err := run(*cfgFileArg); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	log.Println("Starting nightlight service process")
	if err := startServiceProcess(*cfgFileArg); err != nil {
		log.Fatal(err)
	}

}
