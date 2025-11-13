package notify

import (
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

func Beep() {
	f, err := os.Open("beep.mp3")
	if err != nil {
		panic("Failed to retrieve beep.mp3")
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		panic("Failed to decode mp3")
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))
	<-done
}
