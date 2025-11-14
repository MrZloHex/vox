package audio

import (
	"errors"
	"math"
	"time"

	"github.com/gordonklaus/portaudio"
)

type Recorder struct{}

func NewRecorder() *Recorder { return &Recorder{} }

func (r *Recorder) Init() error {
	return portaudio.Initialize()
}

func (r *Recorder) Close() {
	portaudio.Terminate()
}

func (r *Recorder) RecordAuto() ([]float32, error) {
	const (
		sampleRate       = 16000
		frameSize        = 320                             // 20ms
		silenceThreshRMS = 0.015                           // tune if needed
		silenceDuration  = 600 * float32(time.Millisecond) // 600ms
		maxLengthSeconds = 10
	)

	buf := make([]float32, frameSize)
	out := make([]float32, 0, sampleRate*3)

	stream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, len(buf), buf)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return nil, err
	}
	defer stream.Stop()

	var (
		speaking      bool
		silenceFrames int
	)

	maxFrames := maxLengthSeconds * sampleRate / frameSize

	for i := 0; i < maxFrames; i++ {
		if err := stream.Read(); err != nil {
			return nil, err
		}

		rms := frameRMS(buf)

		if rms > silenceThreshRMS {
			speaking = true
			silenceFrames = 0
			out = append(out, buf...)
		} else {
			if speaking {
				silenceFrames++
				if float32(silenceFrames)*20 >= silenceDuration {
					break
				}
				out = append(out, buf...)
			}
		}
	}

	return out, nil
}

func (r *Recorder) RecordUntil(stop <-chan struct{}, maxDur time.Duration) ([]float32, error) {
	const (
		sampleRate = 16000
	)

	if maxDur <= 0 {
		maxDur = 15 * time.Second
	}

	const frameSize = 1024

	buf := make([]float32, frameSize)

	stream, err := portaudio.OpenDefaultStream(
		1, // in
		0, // no out
		float64(sampleRate),
		len(buf),
		buf,
	)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return nil, err
	}
	defer stream.Stop()

	deadline := time.Now().Add(maxDur)
	out := make([]float32, 0, int(float64(sampleRate)*maxDur.Seconds()))

	for {
		// проверяем таймаут
		if time.Now().After(deadline) {
			break
		}

		// проверяем, не попросили ли стоп
		select {
		case <-stop:
			// второй триггер — заканчиваем запись
			return out, nil
		default:
		}

		if err := stream.Read(); err != nil {
			return nil, err
		}

		out = append(out, buf...)
	}

	if len(out) == 0 {
		return nil, errors.New("no audio recorded")
	}

	return out, nil
}

func frameRMS(f []float32) float64 {
	var s float64
	for _, x := range f {
		s += float64(x * x)
	}
	return math.Sqrt(s / float64(len(f)))
}
