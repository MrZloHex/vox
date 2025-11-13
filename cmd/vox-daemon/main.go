package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	cli "github.com/spf13/pflag"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"vox/internal/audio"
	"vox/internal/ipc"
	"vox/internal/nlu"
	"vox/internal/tts"
	"vox/pkg/stt"

	"golang.org/x/net/proxy"
)

func newSocksClient(socksAddr string) (*http.Client, error) {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}, nil
}

func main() {
	fmt.Println("[vox] starting daemon...")

	envFile := cli.StringP("env", "e", ".env", "Env file path")
	cli.Parse()

	godotenv.Load(*envFile)
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY not set")
	}

	httpClient, err := newSocksClient("127.0.0.1:8888")
	if err != nil {
		panic(err)
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
	)

	// --- audio ---
	rec := audio.NewRecorder()
	if err := rec.Init(); err != nil {
		panic(fmt.Errorf("audio init: %w", err))
	}
	defer rec.Close()

	// --- whisper ---
	whisper, err := stt.NewTranscriber("third_party/whisper.cpp/models/ggml-medium.bin")
	if err != nil {
		panic(fmt.Errorf("whisper load: %w", err))
	}
	defer whisper.Close()

	fmt.Println("[vox] daemon ready")

	// --- IPC server ---
	if err := ipc.StartServer(func(msg ipc.ControlMessage) {
		switch msg.Cmd {
		case "trigger":
			handleTrigger(rec, whisper, client)
		default:
			fmt.Println("[vox] unknown command:", msg.Cmd)
		}
	}); err != nil {
		panic(fmt.Errorf("ipc server: %w", err))
	}

	// Block forever
	select {}
}

func handleTrigger(rec *audio.Recorder, tr *stt.Transcriber, api openai.Client) {
	fmt.Println("[vox] trigger received")

	// --- record with silence detection ---
	pcm, err := rec.RecordAuto()
	if err != nil {
		fmt.Println("[vox] record error:", err)
		return
	}
	fmt.Printf("[vox] recorded %d samples\n", len(pcm))

	// --- transcribe ---
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	res, err := tr.TranscribePCM(ctx, pcm, stt.Options{
		Language: "auto",
		Threads:  0,
	})
	if err != nil {
		fmt.Println("[vox] whisper error:", err)
		return
	}

	fmt.Println("[vox] transcript:", res.Text)

	// --- NLU ---
	out, err := nlu.Analyze(api, res.Text)
	if err != nil {
		fmt.Println("[vox] nlu error:", err)
		return
	}

	// --- print result ---
	fmt.Println("──────── VOX ────────")
	fmt.Println("mode:   ", out.Mode)
	fmt.Println("intent: ", out.Intent)
	if len(out.Args) > 0 {
		fmt.Println("args:   ", out.Args)
	}
	if out.Mode == "question" && out.Answer != "" {
		fmt.Println("answer: ", out.Answer)
	}
	fmt.Println("──────────────────────")

	if err := tts.Speak(out.Answer); err != nil {
		fmt.Println("[vox] tts error:", err)
	}
}

/**
package main

import (
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	cli "github.com/spf13/pflag"
	log "log/slog"

	"github.com/openai/openai-go"

	"vox/pkg/protocol"
)

var logLevelMap = map[string]log.Level{
	"debug": log.LevelDebug,
	"info":  log.LevelInfo,
	"warn":  log.LevelWarn,
	"error": log.LevelError,
}

func main() {
	envFile := cli.StringP("env", "e", ".env", "Env file path")
	url := cli.StringP("url", "u", "ws://localhost:8092", "Url of hub")
	logLevel := cli.StringP("log", "l", "info", "Log level")
	cli.Parse()

	log.SetDefault(log.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level: logLevelMap[*logLevel],
	})))

	godotenv.Load(*envFile)
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Error("Failed to get API KEY")
		os.Exit(1)
	}

	ptcl_cfg := protocol.PtclConfig{
		Shard:   "VOX",
		Url:     *url,
		Reconn:  5,
		Timeout: 3 * time.Second,
	}

	ptcl, err := protocol.NewProtocol(ptcl_cfg)
	if err != nil {
		log.Error("Failed to init protocol")
		os.Exit(1)
	}

	go ptcl.Run()

	for {

	}
}

*/
