package audio

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	percentRe = regexp.MustCompile(`(\d+)\s*%`)
)

type streamInfo struct {
	ID      int
	Volume  int
	AppName string
}

type fadeTarget struct {
	id   int
	from int
	to   int
}

// Ducker отвечает за fade in/out ДРУГИХ потоков,
// кроме тех, что помечены application.name из selfNames.
type Ducker struct {
	mu          sync.Mutex
	active      bool
	selfNames   []string    // application.name, которые НЕ трогаем
	originalVol map[int]int // id -> исходный % громкости
	minVolume   int         // минимальная громкость для чужих при duck
}

func NewDucker(selfNames []string, minVolume int) *Ducker {
	if minVolume < 0 {
		minVolume = 0
	}
	if minVolume > 150 {
		minVolume = 150
	}

	d := &Ducker{
		selfNames:   append([]string(nil), selfNames...),
		originalVol: make(map[int]int),
		minVolume:   minVolume,
	}

	return d
}

// DuckOthers: плавно уменьшает громкость всех "чужих" потоков:
// target = current * factor (но не ниже minVolume).
func (d *Ducker) DuckOthers(ctx context.Context, factor float64, duration time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.active {
		return nil
	}

	streams, err := listStreams(ctx)
	if err != nil {
		return fmt.Errorf("listStreams: %w", err)
	}

	d.originalVol = make(map[int]int)

	var targets []fadeTarget

	for _, s := range streams {
		if d.isSelfStream(s) {
			continue
		}

		from := s.Volume

		targetFloat := float64(from) * factor
		if targetFloat < float64(d.minVolume) {
			targetFloat = float64(d.minVolume)
		}
		if targetFloat > 150.0 {
			targetFloat = 150.0
		}

		to := int(math.Round(targetFloat))

		d.originalVol[s.ID] = from

		targets = append(targets, fadeTarget{
			id:   s.ID,
			from: from,
			to:   to,
		})
	}

	if len(targets) == 0 {
		d.active = true
		return nil
	}

	if err := fadeInputs(ctx, targets, duration); err != nil {
		return err
	}

	d.active = true

	return nil
}

// UnduckOthers: плавно возвращает чужие потоки к исходным громкостям.
func (d *Ducker) UnduckOthers(ctx context.Context, duration time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.active {
		return nil
	}

	streams, err := listStreams(ctx)
	if err != nil {
		return fmt.Errorf("listStreams: %w", err)
	}

	curVol := make(map[int]int)

	for _, s := range streams {
		if d.isSelfStream(s) {
			continue
		}
		curVol[s.ID] = s.Volume
	}

	var targets []fadeTarget

	for id, from := range curVol {
		orig, ok := d.originalVol[id]
		if !ok {
			// поток появился уже после duck — можно игнорировать
			continue
		}

		targets = append(targets, fadeTarget{
			id:   id,
			from: from,
			to:   orig,
		})
	}

	if len(targets) > 0 {
		if err := fadeInputs(ctx, targets, duration); err != nil {
			return err
		}
	}

	d.originalVol = make(map[int]int)
	d.active = false

	return nil
}

func (d *Ducker) isSelfStream(s streamInfo) bool {
	for _, name := range d.selfNames {
		if s.AppName == name {
			return true
		}
	}

	return false
}

// fadeInputs делает ступенчатый fade для набора sink-input'ов.
func fadeInputs(ctx context.Context, targets []fadeTarget, duration time.Duration) error {
	if duration <= 0 {
		// сразу выставляем целевые значения
		for _, t := range targets {
			if err := setSinkInputVolume(ctx, t.id, t.to); err != nil {
				return fmt.Errorf("set volume id=%d: %w", t.id, err)
			}
		}

		return nil
	}

	const minStepDuration = 10 * time.Millisecond

	steps := int(duration / minStepDuration)
	if steps < 1 {
		steps = 1
	}

	stepDuration := duration / time.Duration(steps)

	for i := 0; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tFrac := float64(i) / float64(steps)

		for _, s := range targets {
			delta := s.to - s.from
			vFloat := float64(s.from) + float64(delta)*tFrac
			v := int(math.Round(vFloat))

			if err := setSinkInputVolume(ctx, s.id, v); err != nil {
				return fmt.Errorf("set volume id=%d: %w", s.id, err)
			}
		}

		if i < steps {
			time.Sleep(stepDuration)
		}
	}

	return nil
}

// --- pactl helpers ---

func listStreams(ctx context.Context) ([]streamInfo, error) {
	cmd := exec.CommandContext(ctx, "pactl", "list", "sink-inputs")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pactl list sink-inputs: %w", err)
	}

	text := string(out)
	parts := strings.Split(text, "Sink Input #")
	if len(parts) <= 1 {
		return nil, nil
	}

	var res []streamInfo

	for i := 1; i < len(parts); i++ {
		block := parts[i]

		newline := strings.IndexByte(block, '\n')
		if newline <= 0 {
			continue
		}

		idStr := strings.TrimSpace(block[:newline])
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}

		body := block[newline+1:]

		lines := strings.Split(body, "\n")

		s := streamInfo{
			ID: id,
		}

		for _, line := range lines {
			line = strings.TrimSpace(line)

			if strings.HasPrefix(line, "Volume:") && s.Volume == 0 {
				m := percentRe.FindStringSubmatch(line)
				if len(m) >= 2 {
					v, err := strconv.Atoi(m[1])
					if err == nil {
						s.Volume = v
					}
				}
			}

			if strings.HasPrefix(line, "application.name =") && s.AppName == "" {
				// application.name = "Firefox"
				idx := strings.Index(line, "\"")
				if idx >= 0 {
					line = line[idx+1:]
					idx2 := strings.Index(line, "\"")
					if idx2 >= 0 {
						s.AppName = line[:idx2]
					}
				}
			}
		}

		if s.Volume == 0 && s.AppName == "" {
			continue
		}

		res = append(res, s)
	}

	return res, nil
}

func setSinkInputVolume(ctx context.Context, id int, percent int) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 150 {
		percent = 150
	}

	arg := fmt.Sprintf("%d%%", percent)

	cmd := exec.CommandContext(ctx, "pactl", "set-sink-input-volume", strconv.Itoa(id), arg)

	return cmd.Run()
}
