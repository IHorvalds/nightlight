package service_test

import (
	"nightlight/internal/service"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestFileLockWithReader(t *testing.T) {
	const path = "/tmp/service_test.lock"

	if os.Getenv("GO_WANT_HELPER_PROCESS") == "" {

		f, err := service.CreatePidFile(path)
		if err != nil {
			t.Fatalf("%s", err.Error())
		}

		cmd := exec.Command(os.Args[0], "-test.run=^TestFileLockWithReader$")
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		out, err := cmd.CombinedOutput()
		if len(out) > 0 || err != nil {
			t.Fatalf("child process: %q, %v", out, err)
		}
		f.Release()
	} else {
		b, err := os.ReadFile(path)
		if err != nil {

		}
		pid, err := strconv.ParseInt(string(b), 10, 32)
		if err != nil {
			t.Fatalf("failed to parse pid: %s", err.Error())
		}

		if pid != int64(os.Getppid()) {
			t.Fatalf("wrong pid in file %d != %d", pid, os.Getppid())
		}

		os.Exit(0)
	}
}

func TestExclusivity(t *testing.T) {
	const path = "/tmp/service_test.lock"

	if os.Getenv("GO_WANT_HELPER_PROCESS") == "" {

		f, err := service.CreatePidFile(path)
		if err != nil {
			t.Fatalf("%s", err.Error())
		}

		cmd := exec.Command(os.Args[0], "-test.run=^TestExclusivity$")
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		out, err := cmd.CombinedOutput()
		if len(out) > 0 || err != nil {
			t.Fatalf("child process: %q, %v", out, err)
		}
		f.Release()
	} else {
		f, err := service.CreatePidFile(path)
		if err == nil {
			t.Fatal("overwrote pid file of another process")
		}

		if f != nil {
			f.Release()
			t.Fatal("Returned file was not nil")
		}

		os.Exit(0)
	}
}
