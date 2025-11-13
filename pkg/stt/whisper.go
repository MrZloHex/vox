package stt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"time"

	//bind "github.com/ggerganov/whisper.cpp/bindings/go"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type Options struct {
	Language        string        // e.g. "auto", "en", "ru"
	TranslateToEn   bool          // if true, translate non-EN -> EN
	Threads         int           // <=0 => NumCPU()
	InitialPrompt   string        // optional system/prefix prompt
	TokenTimestamps bool          // include per-token timestamps
	MaxTokens       uint          // 0 = no limit
	MaxSegmentChars uint          // 0 = default
	BeamSize        int           // 0 = default (greedy); >0 enables beam search
	AudioCtx        uint          // encoder audio ctx size; 0 = default
	SplitOnWord     bool          // split on word boundaries
	EntropyThold    float32       // 0 = default
	TokenSumThold   float32       // 0 = default
	Temperature     float32       // 0 = default
	TemperatureStep float32       // 0 = default
	Offset          time.Duration // start offset (optional)
	Duration        time.Duration // max duration (optional)
}

type Segment struct {
	Text     string
	StartSec float64
	EndSec   float64
}

type Result struct {
	Text     string
	Segments []Segment
	Language string // detected or forced
}

type Transcriber struct {
	model whisper.Model // interface, not pointer
}

func NewTranscriber(modelPath string) (*Transcriber, error) {
	if modelPath == "" {
		return nil, errors.New("empty model path")
	}
	m, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}
	return &Transcriber{model: m}, nil
}

func (t *Transcriber) Close() error {
	if t.model == nil {
		return nil
	}
	return t.model.Close()
}

// pcm16k must be mono @ 16 kHz, float32 in [-1, 1]
func (t *Transcriber) TranscribePCM(ctx context.Context, pcm16k []float32, opt Options) (Result, error) {
	if t.model == nil {
		return Result{}, errors.New("nil model")
	}
	if len(pcm16k) == 0 {
		return Result{}, errors.New("no audio samples provided")
	}

	wctx, err := t.model.NewContext()
	if err != nil {
		return Result{}, fmt.Errorf("new context: %w", err)
	}

	// ---- configure context (see pkg docs) ----
	if opt.Language == "" {
		opt.Language = "auto"
	}
	if err := wctx.SetLanguage(opt.Language); err != nil {
		return Result{}, fmt.Errorf("set language: %w", err)
	}
	wctx.SetTranslate(opt.TranslateToEn)

	if opt.Offset > 0 {
		wctx.SetOffset(opt.Offset)
	}
	if opt.Duration > 0 {
		wctx.SetDuration(opt.Duration)
	}

	threads := opt.Threads
	if threads <= 0 {
		threads = runtime.NumCPU()
	}
	wctx.SetThreads(uint(threads))

	if opt.SplitOnWord {
		wctx.SetSplitOnWord(true)
	}
	if opt.TokenTimestamps {
		wctx.SetTokenTimestamps(true)
	}
	if opt.MaxTokens > 0 {
		wctx.SetMaxTokensPerSegment(opt.MaxTokens)
	}
	if opt.MaxSegmentChars > 0 {
		wctx.SetMaxSegmentLength(opt.MaxSegmentChars)
	}
	if opt.AudioCtx > 0 {
		wctx.SetAudioCtx(opt.AudioCtx)
	}
	if opt.BeamSize > 0 {
		wctx.SetBeamSize(opt.BeamSize) // enable beam search by setting >0
	}
	if opt.EntropyThold != 0 {
		wctx.SetEntropyThold(opt.EntropyThold)
	}
	if opt.TokenSumThold != 0 {
		wctx.SetTokenSumThreshold(opt.TokenSumThold)
	}
	if opt.InitialPrompt != "" {
		wctx.SetInitialPrompt(opt.InitialPrompt)
	}
	if opt.Temperature != 0 {
		wctx.SetTemperature(opt.Temperature)
	}
	if opt.TemperatureStep != 0 {
		wctx.SetTemperatureFallback(opt.TemperatureStep)
	}

	// ---- run transcription ----
	// We pass nil for callbacks here; you can add live segment/progress later.
	if err := wctx.Process(pcm16k, nil, nil, nil); err != nil {
		return Result{}, fmt.Errorf("process: %w", err)
	}

	var (
		segs     []Segment
		fullText string
	)
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		s, err := wctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("next segment: %w", err)
		}
		segs = append(segs, Segment{
			Text:     s.Text,
			StartSec: s.Start.Seconds(),
			EndSec:   s.End.Seconds(),
		})
		if fullText == "" {
			fullText = s.Text
		} else {
			fullText += " " + s.Text
		}
	}

	lang := wctx.DetectedLanguage()
	if lang == "" {
		lang = wctx.Language()
	}

	return Result{
		Text:     fullText,
		Segments: segs,
		Language: lang,
	}, nil
}
