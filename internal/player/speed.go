package player

import (
	"time"

	"netease-music-mygo/internal/netease"
)

// speedOf is how fast t plays: an episode at PodcastSpeed, a song as it is.
func (p *Player) speedOf(t *netease.Track) float64 {
	if t == nil || t.ProgramID == 0 || p.PodcastSpeed <= 0 {
		return 1
	}
	return min(max(p.PodcastSpeed, 0.5), 3)
}

// speed is how fast the track playing plays.
func (p *Player) speed() float64 {
	if p.curSpeed <= 0 {
		return 1
	}
	return p.curSpeed
}

// heard is where in what plays a moment t of the track is: sooner at a
// speed above one.
func (p *Player) heard(t time.Duration) time.Duration {
	return time.Duration(float64(t) / p.speed())
}

// Speed returns how fast the track playing plays.
func (p *Player) Speed() float64 { return p.speed() }

// SetPodcastSpeed has episodes play at speed, the one playing too, from
// where it is.
func (p *Player) SetPodcastSpeed(speed float64) {
	p.PodcastSpeed = speed
	if p.out == nil || p.stretcher == nil || p.track == nil {
		return
	}
	next := p.speedOf(p.track)
	if next == p.speed() {
		return
	}
	at := p.Position()
	p.stretcher.setSpeed(next)
	p.curSpeed = next
	// The seek applies the speed: frames of what plays count from the
	// start at the speed now.
	p.out.Seek(p.heard(at))
	p.sync()
}

// tellListened tells Listened how long the track playing was listened to, once a
// load: as it ends, or as the listener moves on.
func (p *Player) tellListened(ended bool) {
	if p.Listened == nil || p.track == nil || p.out == nil || p.heardGen == p.gen {
		return
	}
	p.heardGen = p.gen
	p.Listened(*p.track, p.Position(), ended)
}
