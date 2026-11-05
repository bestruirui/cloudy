package player

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"netease-music-mygo/internal/netease"
)

// retryLoad keeps the listener's selection loading until a temporary outage
// clears. A new selection or Pause invalidates even an already posted timer.
func (p *Player) retryLoad(skipped, direction int, err error) bool {
	if !p.playing {
		return false
	}
	wait, ok := retryWait(err, p.retries)
	if !ok {
		return false
	}
	gen := p.gen
	p.loading, p.err = true, ""
	p.retryTimer = time.AfterFunc(wait, func() {
		p.post(func() {
			if gen != p.gen || !p.playing {
				return
			}
			p.retryTimer = nil
			p.loadAttempt(p.at, skipped, true, p.pending, direction, p.retries+1)
		})
	})
	p.sync()
	return true
}

func retryWait(err error, retries int) (time.Duration, bool) {
	if errors.Is(err, context.Canceled) || errors.Is(err, netease.ErrUnavailable) {
		return 0, false
	}
	var network net.Error
	temporary := errors.As(err, &network) || errors.Is(err, io.ErrUnexpectedEOF)
	var api *netease.StreamError
	if errors.As(err, &api) {
		// A response code or a missing address alone cannot establish a
		// permanent failure: the next request can return a playable URL.
		temporary = true
	}
	var status *streamHTTPError
	if errors.As(err, &status) {
		temporary = status.code == 408 || status.code == 429 || status.code >= 500
		// A signed CDN URL can expire; reopening asks for a fresh one.
		if !temporary && status.code != 403 {
			return 0, false
		}
	}
	// Unknown decode failures get a few fresh copies, rather than keeping
	// a permanently broken audio file in a tight reconnect loop.
	if !temporary && retries >= 3 {
		return 0, false
	}
	return 500 * time.Millisecond * time.Duration(1<<min(retries, 4)), true
}
