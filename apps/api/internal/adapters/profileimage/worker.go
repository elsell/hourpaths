package profileimage

import (
	"bytes"
	"context"
	"encoding/json"

	"io"
	"os/exec"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

var ErrBusy = ports.ErrUnavailable

const WorkerArgument = "profile-image-worker"
const workerWireLimit = 15_000_000
const outputLimit = 2_000_000

type request struct {
	Data []byte
	Crop *profilepicture.Crop
}

// Worker permits one active child per API process, and never queues upload bytes.
// The child has no inherited credentials or configuration and is killed at timeout.
type Worker struct {
	executable string
	args       []string
	admission  chan struct{}
}

func NewWorker(executable string, args ...string) *Worker {
	return &Worker{executable: executable, args: append([]string(nil), args...), admission: make(chan struct{}, 1)}
}
func (w *Worker) Process(ctx context.Context, data []byte, crop *profilepicture.Crop) (profilepicture.Prepared, error) {
	if len(data) == 0 || len(data) > profilepicture.MaxUploadBytes {
		return profilepicture.Prepared{}, ports.ErrInvalidArgument
	}
	select {
	case w.admission <- struct{}{}:
		defer func() { <-w.admission }()
	default:
		return profilepicture.Prepared{}, ErrBusy
	}
	encoded, err := json.Marshal(request{Data: data, Crop: crop})
	if err != nil {
		return profilepicture.Prepared{}, ports.ErrInvalidArgument
	}
	deadline, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(deadline, w.executable, w.args...)
	command.Env = []string{"GOMAXPROCS=1", "GOMEMLIMIT=384MiB"}
	command.Stdin = bytes.NewReader(encoded)
	output := &boundedBuffer{remaining: outputLimit}
	command.Stdout = output
	// Do not expose decoder diagnostics or input details in an HTTP error/log.
	if err = command.Run(); err != nil {
		return profilepicture.Prepared{}, ports.ErrInvalidArgument
	}
	var result profilepicture.Prepared
	if json.Unmarshal(output.Bytes(), &result) != nil || len(result.JPEG) == 0 || result.Width < 1 || result.Height < 1 || result.Width > profilepicture.PreviewEdge || result.Height > profilepicture.PreviewEdge {
		return profilepicture.Prepared{}, ports.ErrInvalidArgument
	}
	return result, nil
}

type boundedBuffer struct {
	bytes.Buffer
	remaining int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.remaining {
		return 0, ErrInvalid
	}
	b.remaining -= len(data)
	return b.Buffer.Write(data)
}

// RunWorker is the stdio protocol entry point, called before server bootstrap.
func RunWorker(input io.Reader, output io.Writer) error {
	if err := limitWorkerMemory(); err != nil {
		return err
	}
	decoder := json.NewDecoder(io.LimitReader(input, workerWireLimit))
	decoder.DisallowUnknownFields()
	var req request
	if err := decoder.Decode(&req); err != nil {
		return ErrInvalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	result, err := Transform(req.Data, req.Crop)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}
