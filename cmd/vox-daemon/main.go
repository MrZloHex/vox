package main

import (
	"context"
	"os"
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
			handleTrigger(rec, whisper, client)
		default:
			log.Warn("Unknown command", "cmd", msg.Cmd)
		}
	}); err != nil {
		log.Error("Failed ipc server", "err", err)
		os.Exit(1)
	}

	select {}
}

func handleTrigger(rec *audio.Recorder, tr *stt.Transcriber, api openai.Client) {
	notify.Beep()
	notify.SwayNotify("Listening...")

	log.Info("Starting listening")

	pcm, err := rec.RecordAuto()
	if err != nil {
		log.Error("Failed to recotd", "err", err)
		return
	}

	log.Info("Recorded", "samples", len(pcm))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := tr.TranscribePCM(ctx, pcm, stt.Options{
		Language: "auto",
		Threads:  0,
	})
	if err != nil {
		log.Error("Failed to transribe", "err", err)
		return
	}

	log.Info("Transcribed", "text", res.Text)

	out, err := nlu.Analyze(api, res.Text)
	if err != nil {
		log.Error("Failed to call API", "err", err)
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

	if err := tts.Speak(out.Answer); err != nil {
		log.Error("Failed to voice out", "err", err)
		return
	}
}
