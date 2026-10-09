package profileimage

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestImageWorkerProcess(t *testing.T) {
	if os.Args[len(os.Args)-1] != WorkerArgument {
		return
	}
	if err := RunWorker(os.Stdin, os.Stdout); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
func TestIsolatedImageWorker(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(executable, "-test.run=^TestImageWorkerProcess$", "--", WorkerArgument)
	data, err := os.ReadFile("testdata/basic.heic")
	if err != nil {
		t.Fatal(err)
	}
	result, err := worker.Process(context.Background(), data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Width != 320 || result.Height != 240 {
		t.Fatalf("wrong result %dx%d", result.Width, result.Height)
	}
	if _, err = worker.Process(context.Background(), []byte("not an image"), nil); err == nil {
		t.Fatal("accepted corrupt image")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = worker.Process(ctx, data, nil); err == nil {
		t.Fatal("ignored request cancellation")
	}
}

func TestWorkerAllocationLimitProcess(t *testing.T) {
	if os.Args[len(os.Args)-1] != "allocation-limit-probe" {
		return
	}
	if err := limitWorkerMemory(); err != nil {
		os.Exit(2)
	}
	data := make([]byte, 512<<20)
	for i := range data {
		data[i] = 1
	}
	os.Exit(int(data[0]) - 1)
}
func TestWorkerHardAllocationLimit(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestWorkerAllocationLimitProcess$", "--", "allocation-limit-probe")
	command.Env = []string{"GOMAXPROCS=1", "GOMEMLIMIT=384MiB"}
	var output bytes.Buffer
	command.Stderr = &output
	err = command.Run()
	if err == nil || ctx.Err() != nil || !strings.Contains(output.String(), "out of memory") {
		t.Fatalf("allocation was not rejected by worker memory limit: %v %s", err, output.String())
	}
}
