package audioconv

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
	popus "github.com/pekim/opus"
)

type Options struct {
	MaxSamples int
}

func ConvertFileToPCM16k(_ context.Context, path string, opt Options) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav":
		return decodeWAVTo16k(f, opt)
	case ".mp3":
		return decodeMP3To16k(f, opt)
	case ".ogg", ".oga":
		if s, err := decodeOggVorbisTo16k(f, opt); err == nil {
			return s, nil
		} else {
			if _, e2 := f.Seek(0, io.SeekStart); e2 == nil {
				if s2, e3 := decodeOggOpusTo16k(f, opt); e3 == nil {
					return s2, nil
				} else {
					return nil, fmt.Errorf("cannot decode .ogg/.oga as Opus: %w", e3)
				}
			}
			return nil, fmt.Errorf("cannot decode .ogg/.oga as Vorbis or Opus")
		}
	default:
		// Quick sniff
		br := bufio.NewReader(f)
		magic, _ := br.Peek(4)
		_, _ = f.Seek(0, io.SeekStart)
		switch string(magic) {
		case "RIFF":
			return decodeWAVTo16k(f, opt)
		case "OggS":
			if s, err := decodeOggVorbisTo16k(f, opt); err == nil {
				return s, nil
			}
			if _, e2 := f.Seek(0, io.SeekStart); e2 == nil {
				if s2, e3 := decodeOggOpusTo16k(f, opt); e3 == nil {
					return s2, nil
				}
			}
			return nil, fmt.Errorf("cannot decode Ogg container (Vorbis/Opus failed; build with -tags opus for Opus)")
		default:
			return nil, fmt.Errorf("unsupported format: %s (supported: wav/mp3/ogg-vorbis[/opus])", ext)
		}
	}
}

func decodeWAVTo16k(r io.ReadSeeker, opt Options) ([]float32, error) {
	dec := wav.NewDecoder(r)
	if !dec.IsValidFile() {
		return nil, errors.New("invalid wav")
	}
	pb, err := dec.FullPCMBuffer()
	if err != nil || pb == nil || pb.Data == nil {
		if err == nil {
			err = errors.New("empty wav")
		}
		return nil, err
	}

	bd := int(dec.BitDepth)
	if bd == 0 {
		bd = 16
	}
	x := intSliceToFloat32(pb.Data, bd)

	ch := 1
	sr := 44100
	if pb.Format != nil {
		if pb.Format.NumChannels > 0 {
			ch = pb.Format.NumChannels
		}
		if pb.Format.SampleRate > 0 {
			sr = pb.Format.SampleRate
		}
	}
	if ch > 1 {
		x = downmixInterleaved(x, ch)
	}
	if sr != 16000 {
		x = resampleLinear(x, sr, 16000)
	}
	if opt.MaxSamples > 0 && len(x) > opt.MaxSamples {
		x = x[:opt.MaxSamples]
	}
	return x, nil
}

func decodeMP3To16k(r io.Reader, opt Options) ([]float32, error) {
	dec, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, err
	}
	var raw bytes.Buffer
	if _, err := io.Copy(&raw, dec); err != nil {
		return nil, err
	}
	ints := make([]int16, raw.Len()/2)
	if err := binary.Read(bytes.NewReader(raw.Bytes()), binary.LittleEndian, &ints); err != nil {
		return nil, err
	}
	x := int16SliceToFloat32(ints)
	x = downmixInterleaved(x, 2) // mp3 decoder outputs stereo

	sr := dec.SampleRate()
	if sr <= 0 {
		sr = 44100
	}
	if sr != 16000 {
		x = resampleLinear(x, sr, 16000)
	}
	if opt.MaxSamples > 0 && len(x) > opt.MaxSamples {
		x = x[:opt.MaxSamples]
	}
	return x, nil
}

func decodeOggVorbisTo16k(r io.Reader, opt Options) ([]float32, error) {
	pcm, fmt, err := oggvorbis.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if fmt == nil || fmt.Channels <= 0 || fmt.SampleRate <= 0 {
		return nil, errors.New("invalid ogg/vorbis stream")
	}
	var x []float32
	if fmt.Channels == 1 {
		x = pcm
	} else {
		x = downmixInterleaved(pcm, fmt.Channels)
	}
	if fmt.SampleRate != 16000 {
		x = resampleLinear(x, fmt.SampleRate, 16000)
	}
	if opt.MaxSamples > 0 && len(x) > opt.MaxSamples {
		x = x[:opt.MaxSamples]
	}
	return x, nil
}

func decodeOggOpusTo16k(r io.Reader, opt Options) ([]float32, error) {
	var rs io.ReadSeeker
	switch v := r.(type) {
	case io.ReadSeeker:
		rs = v
	case *os.File:
		rs = v
	default:
		// fallback: buffer into memory
		b, err := io.ReadAll(v)
		if err != nil {
			return nil, err
		}
		rs = bytes.NewReader(b)
	}

	dec, err := popus.NewDecoder(rs)
	if err != nil {
		return nil, err
	}
	defer dec.Destroy()

	ch := dec.ChannelCount() // 1=mono, 2=stereo, ...
	if ch <= 0 {
		ch = 1
	}

	// Read chunks as int16 PCM @ 48k
	var (
		pcm48 []float32
		buf   = make([]int16, 48_000*ch/2) // ~0.5s of audio
	)
	for {
		n, err := dec.Read(buf) // n = samples per channel
		if n > 0 {
			// Slice actual written samples (interleaved if ch>1)
			w := buf[:n*ch]
			// Convert to float32 [-1,1]
			tmp := int16SliceToFloat32(w)
			pcm48 = append(pcm48, tmp...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	if len(pcm48) == 0 {
		return nil, nil
	}

	// Downmix interleaved to mono when needed
	if ch > 1 {
		pcm48 = downmixInterleaved(pcm48, ch)
	}

	// Resample 48k -> 16k
	out := resampleLinear(pcm48, 48000, 16000)

	if opt.MaxSamples > 0 && len(out) > opt.MaxSamples {
		out = out[:opt.MaxSamples]
	}
	return out, nil
}

// helpers

func intSliceToFloat32(data []int, bitDepth int) []float32 {
	out := make([]float32, len(data))
	scale := 1.0 / float64(int64(1)<<(bitDepth-1))
	for i, v := range data {
		out[i] = float32(clamp(float64(v)*scale, -1.0, 1.0))
	}
	return out
}

func int16SliceToFloat32(data []int16) []float32 {
	out := make([]float32, len(data))
	const scale = 1.0 / 32768.0
	for i, v := range data {
		out[i] = float32(float64(v) * scale)
	}
	return out
}

func downmixInterleaved(in []float32, channels int) []float32 {
	if channels <= 1 {
		return in
	}
	nFrames := len(in) / channels
	out := make([]float32, nFrames)
	for i := 0; i < nFrames; i++ {
		sum := 0.0
		base := i * channels
		for c := 0; c < channels; c++ {
			sum += float64(in[base+c])
		}
		out[i] = float32(sum / float64(channels))
	}
	return out
}

func resampleLinear(in []float32, inSR, outSR int) []float32 {
	if inSR == outSR || len(in) == 0 {
		return in
	}
	ratio := float64(outSR) / float64(inSR)
	outN := int(math.Ceil(float64(len(in)) * ratio))
	out := make([]float32, outN)
	for i := 0; i < outN; i++ {
		src := float64(i) / ratio
		i0 := int(math.Floor(src))
		i1 := i0 + 1
		if i0 >= len(in) {
			out[i] = in[len(in)-1]
			continue
		}
		if i1 >= len(in) {
			out[i] = in[i0]
			continue
		}
		a := float32(src - float64(i0))
		out[i] = in[i0]*(1-a) + in[i1]*a
	}
	return out
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
