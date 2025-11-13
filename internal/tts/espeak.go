package tts

/*
#cgo LDFLAGS: -lespeak-ng
#include <stdlib.h>
#include <espeak-ng/speak_lib.h>

int
espeak_say(const char *text)
{
	if (!text)
	{ return -1; }

	espeak_Initialize(AUDIO_OUTPUT_SYNCH_PLAYBACK, 500, NULL, 0);
	espeak_VOICE specs = { .languages = "ru" };
	espeak_SetVoiceByProperties(&specs);

	espeak_Synth(text, 500, 0, 0, 0, espeakCHARS_AUTO, NULL, NULL);
	espeak_Synchronize();
	espeak_Terminate();

	return 0;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func Speak(text string) error {
    if text == "" {
        return nil
    }

    ctext := C.CString(text)
    defer C.free(unsafe.Pointer(ctext))

    rc := C.espeak_say(ctext)
    if rc != 0 {
        return fmt.Errorf("espeak_say failed: %d", int(rc))
    }

    return nil
}
