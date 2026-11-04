package app

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"netease-music-mygo/internal/player"
)

type waitingCaptionAudio struct {
	ctx    context.Context
	closed chan struct{}
}

func (s *waitingCaptionAudio) SampleRate() int             { return 44100 }
func (s *waitingCaptionAudio) Channels() int               { return 2 }
func (s *waitingCaptionAudio) Read([]float32) (int, error) { return 0, s.ctx.Err() }
func (s *waitingCaptionAudio) SetPosition(int64) error     { <-s.ctx.Done(); return s.ctx.Err() }
func (s *waitingCaptionAudio) Close() error                { close(s.closed); return nil }

// Starting captions returns before a stalled seek; disabling captions cancels and closes it.
func TestFixCaptionSeekOffUIThread(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := newTestApp()
		closed := make(chan struct{})
		a.transcribeAudio(7, time.Minute, 0, "unused", func(ctx context.Context) (player.Audio, error) {
			return &waitingCaptionAudio{ctx: ctx, closed: closed}, nil
		})
		synctest.Wait()
		if a.captions.work == nil {
			t.Fatal("caption job was not registered before waiting")
		}
		select {
		case <-closed:
			t.Fatal("seek did not wait")
		default:
		}
		a.pauseCaptions()
		synctest.Wait()
		select {
		case <-closed:
		default:
			t.Fatal("cancelling captions left its seek blocked")
		}
	})
}
