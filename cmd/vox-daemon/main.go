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
	_ "vox/internal/tts"
	"vox/pkg/protocol"
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
	url := cli.StringP("url", "u", "ws://192.168.0.69:8092", "Url of hub")
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

	ptcl_cfg := protocol.PtclConfig{
		Shard:   "VOX",
		Url:     *url,
		Reconn:  5,
		Timeout: 3 * time.Second,
	}

	ptcl, err := protocol.NewProtocol(ptcl_cfg)
	if err != nil {
		log.Error("Failed to init protocol", "err", err)
		os.Exit(1)
	}
	go ptcl.Run()

	log.Debug("Loaded protocol")

	log.Info("Boot up - successful")

	if err := ipc.StartServer(func(msg ipc.ControlMessage) {
		switch msg.Cmd {
		case "trigger":
			handleToggleTrigger(rec, whisper, client, ptcl)
		default:
			log.Warn("Unknown command", "cmd", msg.Cmd)
		}
	}); err != nil {
		log.Error("Failed ipc server", "err", err)
		os.Exit(1)
	}

	select {}
}

func handleToggleTrigger(rec *audio.Recorder, tr *stt.Transcriber, api openai.Client, ptcl *protocol.Protocol) {
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

			handleSession(stop, rec, tr, api, ptcl)
		}(stopChan)

	} else {
		if stopChan != nil {
			log.Info("Stopping listening by trigger")
			close(stopChan)
			stopChan = nil
		}
	}
}

func handleSession(stop <-chan struct{}, rec *audio.Recorder, tr *stt.Transcriber, api openai.Client, ptcl *protocol.Protocol) {
	ctx_bg := context.Background()

	d := audio.NewDucker([]string{"MonolithVox"}, 5)
	err := d.DuckOthers(ctx_bg, 0.3, 400*time.Millisecond)
	if err != nil {
		log.Error("Failed to duck outputs", "err", err)
	}
	log.Debug("Ducked audio")

	notify.Beep()
	notify.SwayNotify("Listening...")
	log.Debug("Sent notification")

	log.Info("Starting listening")

	pcm, err := rec.RecordUntil(stop, 20*time.Second)
	if err != nil {
		log.Error("record failed", "err", err)
		return
	}
	log.Debug("Recored audio", "samples", len(pcm))

	err = d.UnduckOthers(ctx_bg, 400*time.Millisecond)
	if err != nil {
		log.Error("Failed to unduck outputs", "err", err)
	}
	log.Debug("Unducked audio")

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
	log.Info("intent: ", "i", out.Intent)
	log.Info("entities:   ", "e", out.Entities)
	log.Info("──────────────────────")

	resp, err := nlu.Dispatch(out, ptcl)
	if err != nil {
		log.Error("Failed to dispatch", "err", err)
	}
	log.Debug("Dispatched request", "resp", resp)

	err = d.DuckOthers(ctx_bg, 0.3, 400*time.Millisecond)
	if err != nil {
		log.Error("Failed to duck outputs", "err", err)
	}
	log.Debug("Ducked audio")

	log.Debug("Speaking out")
	// err = tts.Speak(out.Answer)
	// if err != nil {
	// 	log.Error("Failed to voice out", "err", err)
	// }

	err = d.UnduckOthers(ctx_bg, 400*time.Millisecond)
	if err != nil {
		log.Error("Failed to unduck outputs", "err", err)
	}
	log.Debug("Unducked audio")

	log.Debug("Request handled")
}
