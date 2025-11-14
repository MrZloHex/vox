package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	cli "github.com/spf13/pflag"

	"github.com/lmittmann/tint"
	log "log/slog"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"vox/internal/audio"
	"vox/internal/ipc"
	"vox/internal/nlu"
	"vox/internal/notify"
	"vox/internal/proxy"
	"vox/internal/tts"
	"vox/pkg/stt"
)

var logLevelMap = map[string]log.Level{
	"debug": log.LevelDebug,
	"info":  log.LevelInfo,
	"warn":  log.LevelWarn,
	"error": log.LevelError,
}

var (
	sttMu     sync.Mutex
	recMu     sync.Mutex
	recording bool
	stopChan  chan struct{}
)

func main() {
	envFile := cli.StringP("env", "e", ".env", "Env file path")
	// url := cli.StringP("url", "u", "ws://localhost:8092", "Url of hub")
	proxy_addr := cli.StringP("proxy", "p", "127.0.0.1:8888", "Socks Proxy Address")
	logLevel := cli.StringP("log", "l", "info", "Log level")
	cli.Parse()

	log.SetDefault(log.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level: logLevelMap[*logLevel],
	})))

	log.Info("Booting up")

	godotenv.Load(*envFile)
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Error("OPENAI_API_KEY not set")
		os.Exit(1)
	}

	log.Debug("Loaded API Key")

	httpClient, err := proxy.NewSocksClient(*proxy_addr)
	if err != nil {
		log.Error("Failed to dial socks proxy", "proxy", *proxy_addr, "err", err)
		os.Exit(1)
	}

	log.Debug("Loaded proxy")

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
	)

	rec := audio.NewRecorder()
	err = rec.Init()
	if err != nil {
		log.Error("Failed to init audio", "err", err)
		os.Exit(1)
	}
	defer rec.Close()

	log.Debug("Loaded recorder")

	whisper, err := stt.NewTranscriber("third_party/whisper.cpp/models/ggml-medium.bin")
	if err != nil {
		log.Error("Failed to ini whisper", "err", err)
		os.Exit(1)
	}
	defer whisper.Close()

	log.Debug("Loaded whisper")
	log.Info("Boot up - successful")

	if err := ipc.StartServer(func(msg ipc.ControlMessage) {
		switch msg.Cmd {
		case "trigger":
			handleToggleTrigger(rec, whisper, client)
		default:
			log.Warn("Unknown command", "cmd", msg.Cmd)
		}
	}); err != nil {
		log.Error("Failed ipc server", "err", err)
		os.Exit(1)
	}

	select {}
}

func handleToggleTrigger(rec *audio.Recorder, tr *stt.Transcriber, api openai.Client) {
	recMu.Lock()
	defer recMu.Unlock()

	if !recording {
		log.Info("Warming up records")

		recording = true
		stopChan = make(chan struct{})

		go func(stop <-chan struct{}) {
			defer func() {
				recMu.Lock()
				recording = false
				stopChan = nil
				recMu.Unlock()

				log.Info("Listening finished")
			}()

			handleSession(stop, rec, tr, api)
		}(stopChan)

	} else {
		if stopChan != nil {
			log.Info("Stopping listening by trigger")
			close(stopChan)
			stopChan = nil
		}
	}
}

func handleSession(stop <-chan struct{}, rec *audio.Recorder, tr *stt.Transcriber, api openai.Client) {
	ctx_bg := context.Background()
	d := audio.NewDucker([]string{"MonolithVox"}, 5)
	d.DuckOthers(ctx_bg, 0.3, 400*time.Millisecond)
	defer d.UnduckOthers(ctx_bg, 400*time.Millisecond)

	notify.Beep()
	notify.SwayNotify("Listening...")
	log.Info("Starting listening")
	defer log.Debug("Request handled")

	pcm, err := rec.RecordUntil(stop, 20*time.Second)
	if err != nil {
		log.Error("record failed", "err", err)
		return
	}
	log.Info("Recorded samples", "samples", len(pcm))

	d.UnduckOthers(ctx_bg, 400*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Debug("Start transcripting")

	res, err := tr.TranscribePCM(ctx, pcm, stt.Options{
		Language: "auto",
		Threads:  0,
	})
	if err != nil {
		log.Error("whisper transcribe failed", "err", err)
		return
	}

	log.Info("Transcribed", "text", res.Text, "lang", res.Language)
	log.Debug("Starting analyzing")

	out, err := nlu.Analyze(api, res.Text)
	if err != nil {
		log.Error("nlu failed", "err", err)
		return
	}

	log.Info("──────── VOX ────────")
	log.Info("mode:   ", out.Mode)
	log.Info("intent: ", out.Intent)
	if len(out.Args) > 0 {
		log.Info("args:   ", out.Args)
	}
	if out.Mode == "question" && out.Answer != "" {
		log.Info("answer: ", out.Answer)
	}
	log.Info("──────────────────────")

	/*
		ctxNLU := ctx
		handled, err := mono.Dispatch(ctxNLU, log, nluRes)
		if err != nil {
			log.Error("mono dispatch error", slog.Any("err", err))
		}
		if handled {
			return
		}
	*/

	d.DuckOthers(ctx_bg, 0.3, 400*time.Millisecond)
	log.Debug("Speaking out")

	if err := tts.Speak(out.Answer); err != nil {
		log.Error("Failed to voice out", "err", err)
		return
	}

}
