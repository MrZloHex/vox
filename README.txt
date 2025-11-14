███╗   ███╗ ██████╗ ███╗   ██╗ ██████╗ ██╗     ██╗████████╗██╗  ██╗
████╗ ████║██╔═══██╗████╗  ██║██╔═══██╗██║     ██║╚══██╔══╝██║  ██║
██╔████╔██║██║   ██║██╔██╗ ██║██║   ██║██║     ██║   ██║   ███████║
██║╚██╔╝██║██║   ██║██║╚██╗██║██║   ██║██║     ██║   ██║   ██╔══██║
██║ ╚═╝ ██║╚██████╔╝██║ ╚████║╚██████╔╝███████╗██║   ██║   ██║  ██║
╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝ ╚══════╝╚═╝   ╚═╝   ╚═╝  ╚═╝

  ░▒▓█ _VOX_ █▓▒░
  A voice-first conduit into the Monolith world.
 
  ───────────────────────────────────────────────────────────────
  ▓ OVERVIEW
  **VOX** is the resident speech assistant for the Monolith ecosystem.
  The `vox-daemon` captures audio from your microphone, performs Whisper
  transcription, runs NLU via OpenAI, and answers aloud through eSpeak NG
  while ducking any competing audio. The tiny `vox-ctl` helper toggles a
  capture session over a Unix socket. Trigger, listen, respond.
 
  ───────────────────────────────────────────────────────────────
  ▓ REQUIREMENTS
  ▪ `OPENAI_API_KEY` in the environment (loadable from `.env`)
  ▪ Whisper model file: `third_party/whisper.cpp/models/ggml-medium.bin`
  ▪ System packages: PortAudio, eSpeak NG (`libespeak-ng`), FFmpeg/`ffplay`,
    `notify-send`, and PulseAudio's `pactl`
  ▪ Audio output named `MonolithVox` for ducking exemption (optional)
 
  ───────────────────────────────────────────────────────────────
  ▓ FEATURES
  ▪ Push-to-talk with `vox-ctl` (second trigger stops recording)
  ▪ Automatic audio ducking for everything except the VOX sink
  ▪ Local Whisper transcription (ggml) with configurable threads
  ▪ OpenAI ChatGPT NLU that yields intents, slots, and answers
  ▪ Text-to-speech replies through eSpeak NG plus desktop notifications
 
  ───────────────────────────────────────────────────────────────
  ▓ BUILDING
  Fetch and build Whisper once
  then in root
 
  ```sh
  make build 
  ```
 
  ───────────────────────────────────────────────────────────────
  ▓ RUNNING
  1. Place your Whisper model at `third_party/whisper.cpp/models/ggml-medium.bin`
  2. Create a `.env` with `OPENAI_API_KEY=<your_key>`
  3. Launch the daemon:
 
     ```sh
     ./bin/vox-daemon
     ```
 
     Useful flags: `--env <file>`, `--proxy <host:port>`, `--log {debug|info|warn|error}`.
 
  4. From another terminal, start/stop listening with:
 
     ```sh
     go run ./cmd/vox-ctl
     ```
 
  The daemon writes transcripts and NLU decisions to stdout and speaks the
  answer aloud. A repeat control command stops an active session.
 
  ───────────────────────────────────────────────────────────────
  ▓ FINAL WORDS
  Speak up. VOX is listening.
 
