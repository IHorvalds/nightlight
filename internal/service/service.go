package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path"
	"strconv"
	"sync"
	"syscall"
)

const (
	pidFilename = "nightlight.pid"
)

var lockDir = os.TempDir()

func RunService(cfgFile string) error {
	fi, err := os.Stat(lockDir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return syscall.ENOTDIR
	}

	pidFile := path.Join(lockDir, pidFilename)
	f, err := CreatePidFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to create PID file: %w", err)
	}
	defer func() {
		if err := f.Release(); err != nil {
			log.Printf("Failed to release PID file: %s", err.Error())
		}
	}()

	cfg, err := FromFile(cfgFile)
	if err != nil {
		return err
	}

	if err = cfg.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	stopCh := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	log.Println("Starting the nightlight service")
	var wg sync.WaitGroup
	wg.Add(1)
	go serviceLoop(cfgFile, stopCh, &wg)

	// wait to get a signal
	<-sig
	log.Println("Stopping the nightlight service")
	stopCh <- struct{}{}
	wg.Wait()
	close(stopCh)
	close(sig)

	return nil
}

func StopService() error {

	pidFile := path.Join(lockDir, pidFilename)
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

type LockFile struct {
	path string
	f    *os.File
}

func CreatePidFile(path string) (*LockFile, error) {
	f, err := os.OpenFile(path, syscall.O_CREAT|syscall.O_CLOEXEC|syscall.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}

	flock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: io.SeekStart,
		Start:  0,
		Len:    0,
	}
	err = syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &flock)
	if err != nil {
		f.Close()
		return nil, err
	}

	_, err = f.WriteString(fmt.Sprintf("%d", os.Getpid()))
	if err != nil {
		f.Close()
		return nil, err
	}

	return &LockFile{
		path: path,
		f:    f,
	}, nil
}

func (lf *LockFile) Release() error {
	lf.f.Close()

	return os.Remove(lf.path)
}
