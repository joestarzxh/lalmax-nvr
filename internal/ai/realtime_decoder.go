package ai

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/lalmax-pro/lalmax-nvr/internal/model"
)

type decodedFrameCallback func(pts int64, jpeg []byte)

type realtimeDecoder struct {
	cameraID  string
	codec     model.Format
	cancel    context.CancelFunc
	stdin     io.WriteCloser
	frames    chan encodedFrame
	done      chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool
	latestPTS atomic.Int64
}

type encodedFrame struct {
	pts  int64
	data []byte
}

func newRealtimeDecoder(ctx context.Context, configuredFFmpegPath, cameraID string, codec model.Format, onFrame decodedFrameCallback) (*realtimeDecoder, error) {
	ffmpegPath, err := resolveFFmpegPath(configuredFFmpegPath)
	if err != nil {
		return nil, err
	}
	inputFormat, err := ffmpegInputFormat(codec)
	if err != nil {
		return nil, err
	}

	decoderCtx, cancel := context.WithCancel(ctx)
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-an",
		"-f", inputFormat,
		"-i", "pipe:0",
		"-vf", "fps=2",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	}
	cmd := exec.CommandContext(decoderCtx, ffmpegPath, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open ffmpeg stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open ffmpeg stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start ffmpeg realtime decoder: %w", err)
	}

	d := &realtimeDecoder{
		cameraID: cameraID,
		codec:    codec,
		cancel:   cancel,
		stdin:    stdin,
		frames:   make(chan encodedFrame, 8),
		done:     make(chan struct{}),
	}

	var waitOnce sync.Once
	wait := func() {
		waitOnce.Do(func() {
			_ = cmd.Wait()
			if stderr.Len() > 0 {
				logger.Debug("ffmpeg realtime decoder exited", "camera_id", cameraID, "codec", codec, "stderr", stderr.String())
			}
		})
	}

	go d.writeLoop()
	go d.readLoop(stdout, onFrame)
	go func() {
		defer close(d.done)
		wait()
	}()

	return d, nil
}

func resolveFFmpegPath(configured string) (string, error) {
	if configured != "" {
		if st, err := os.Stat(configured); err == nil && !st.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("configured ffmpeg path not found: %s", configured)
	}
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("ffmpeg not found")
}

func ffmpegInputFormat(codec model.Format) (string, error) {
	switch codec {
	case model.FormatH264:
		return "h264", nil
	case model.FormatH265:
		return "hevc", nil
	default:
		return "", fmt.Errorf("unsupported codec %q", codec)
	}
}

func (d *realtimeDecoder) enqueue(pts int64, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("ffmpeg realtime decoder enqueue ignored after close", "camera_id", d.cameraID)
		}
	}()
	if d.closed.Load() {
		return
	}
	if len(data) == 0 {
		return
	}
	d.latestPTS.Store(pts)
	frame := encodedFrame{pts: pts, data: append([]byte(nil), data...)}
	select {
	case d.frames <- frame:
	default:
		select {
		case <-d.frames:
		default:
		}
		select {
		case d.frames <- frame:
		default:
		}
	}
}

func (d *realtimeDecoder) writeLoop() {
	for frame := range d.frames {
		if _, err := d.stdin.Write(frame.data); err != nil {
			logger.Debug("ffmpeg realtime decoder stdin write failed", "camera_id", d.cameraID, "error", err)
			return
		}
	}
}

func (d *realtimeDecoder) readLoop(stdout io.Reader, onFrame decodedFrameCallback) {
	reader := bufio.NewReader(stdout)
	for {
		jpeg, err := readJPEG(reader)
		if err != nil {
			if err != io.EOF {
				logger.Debug("ffmpeg realtime decoder stdout read failed", "camera_id", d.cameraID, "error", err)
			}
			return
		}
		onFrame(d.latestPTS.Load(), jpeg)
	}
}

func (d *realtimeDecoder) close() {
	d.closeOnce.Do(func() {
		d.closed.Store(true)
		d.cancel()
		close(d.frames)
		_ = d.stdin.Close()
		<-d.done
	})
}

func readJPEG(reader *bufio.Reader) ([]byte, error) {
	var out []byte
	prev := byte(0)
	started := false
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if !started {
			if prev == 0xff && b == 0xd8 {
				started = true
				out = append(out, 0xff, 0xd8)
			}
			prev = b
			continue
		}
		out = append(out, b)
		if prev == 0xff && b == 0xd9 {
			return out, nil
		}
		prev = b
	}
}
