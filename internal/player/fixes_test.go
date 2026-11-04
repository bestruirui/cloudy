package player

import (
	"context"
	"os"
	"testing"
	"testing/synctest"
)

// Cancelling a second decoder while opening must wake it without cancelling playback.
func TestFixCancelSharedOpen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dl := downloaded(nil)
		dl.done = false
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { _, err := openSharedContext(ctx, dl, 600000); done <- err }()
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("opening did not wait for bytes: %v", err)
		default:
		}
		cancel()
		synctest.Wait()
		if err := <-done; err == nil {
			t.Fatal("cancelled opening succeeded")
		}
		if dl.done || dl.err != nil || dl.users != 1 {
			t.Fatalf("cancelling reader affected playback: done=%v err=%v users=%d", dl.done, dl.err, dl.users)
		}
	})
}

// Seeking or reading unbuffered MP3 and FLAC audio can be cancelled for captions alone.
func TestFixCancelSharedAudio(t *testing.T) {
	for _, ext := range []string{"mp3", "flac"} {
		b, err := os.ReadFile("testdata/seek." + ext)
		if err != nil {
			t.Fatal(err)
		}
		for _, operation := range []string{"seek", "read"} {
			t.Run(ext+"/"+operation, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					dl := downloaded(append([]byte(nil), b[:len(b)/4]...))
					dl.size, dl.done = int64(len(b)), false
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					src, err := openSharedContext(ctx, dl, 5000)
					if err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					go func() {
						if operation == "seek" {
							done <- src.SetPosition(int64(src.SampleRate()) * 4)
							return
						}
						buf := make([]float32, 4096)
						for {
							if _, err := src.Read(buf); err != nil {
								done <- err
								return
							}
						}
					}()
					synctest.Wait()
					select {
					case err := <-done:
						t.Fatalf("operation did not wait for missing bytes: %v", err)
					default:
					}
					cancel()
					synctest.Wait()
					<-done // FLAC seeking can stop at its last known frame; Read then fails.
					src.Close()
					if dl.done || dl.err != nil || dl.users != 1 {
						t.Fatal("cancelling captions stopped or leaked the playback download")
					}
					dl.write(b[len(b)/4:])
					dl.done = true
					playback, err := openStream(dl, 5000)
					if err != nil {
						t.Fatal(err)
					}
					defer playback.Close()
					if n, err := playback.Read(make([]float32, 4096)); n == 0 || err != nil {
						t.Fatalf("playback failed after reader cancellation: %d, %v", n, err)
					}
				})
			})
		}
	}
}
