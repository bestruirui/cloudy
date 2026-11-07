package player

import "math"

// The sound placed around the listener's head, for headphones: by how much
// later, and how much softer and duller, it reaches the ear turned from it
// than the ear turned to it, as a head between them makes it. A sphere the
// size of a head (Woodworth's delay, and Brown and Duda's shadow, which
// takes the highs off what goes round it) stands in for a measured one.

// Spatial modes, of Effects.Spatial.
const (
	Spatial8D  = "8d"  // the sound circles the head
	Spatial360 = "360" // the sound before the listener, as from speakers
)

// headRadius is a head's, in metres; sound goes 343 m a second.
const (
	headRadius = 0.0875
	soundSpeed = 343.0
)

// itd is how much later sound from azimuth az, in radians from straight
// ahead, reaches the far ear, in seconds.
func itd(az float64) float64 {
	a := math.Abs(math.Asin(math.Sin(az))) // folded to the front, 0 to π/2
	return headRadius / soundSpeed * (a + math.Sin(a))
}

// shadow takes off the highs the head keeps from the far ear: a one-pole
// filter whose corner, about 1.6 kHz, lowers as the sound goes round.
type shadow struct {
	k, y float64
}

func (s *shadow) set(rate, amount float64) {
	// amount 0 lets all through, 1 shades as far as a sound from the side.
	corner := 20000 * math.Pow(1600.0/20000, amount)
	s.k = 1 - math.Exp(-2*math.Pi*corner/rate)
}

func (s *shadow) run(x float64) float64 {
	s.y += s.k * (x - s.y)
	return s.y
}

// line is a short delay, read at a fractional length.
type line struct {
	buf []float64
	at  int
}

func newLine(n int) line { return line{buf: make([]float64, n)} }

func (l *line) push(x float64) {
	l.buf[l.at] = x
	if l.at++; l.at == len(l.buf) {
		l.at = 0
	}
}

// back reads d frames back, between frames as d says.
func (l *line) back(d float64) float64 {
	n := len(l.buf)
	d = min(max(d, 1), float64(n-1))
	i := int(d)
	f := d - float64(i)
	a := l.buf[(l.at-i+n)%n]
	b := l.buf[(l.at-i-1+n)%n]
	return a + (b-a)*f
}

// orbit circles the sound round the head, once every period, at a speed
// of Orbit: the sound to one side reaches the other ear later and duller,
// and behind, both a little duller, as it does.
type orbit struct {
	rate, angle float64
	in          line
	near, far   [2]shadow // each ear's: as it faces the sound, and away
}

func newOrbit(rate float64) *orbit {
	return &orbit{rate: rate, in: newLine(int(rate*0.002) + 4)}
}

// orbitPeriod is how long a round takes, in seconds: from 16 at speed 0 to
// 4 at speed 1.
func orbitPeriod(speed float64) float64 { return 16 - 12*speed }

func (o *orbit) run(l, r, speed float64) (float64, float64) {
	o.angle += 2 * math.Pi / (orbitPeriod(speed) * o.rate)
	if o.angle > 2*math.Pi {
		o.angle -= 2 * math.Pi
	}
	// What circles is the whole, the mid; the sides stay as a room's.
	mid, side := (l+r)/2, (l-r)/2
	o.in.push(mid)
	s := math.Sin(o.angle) // 1 at the right, -1 at the left
	behind := math.Max(0, -math.Cos(o.angle))
	late := itd(o.angle) * o.rate
	// The ear facing the sound hears it now; the other, later and duller.
	toward, away := o.in.back(1), o.in.back(1+late)
	gainNear := 1.0
	gainFar := 1 - 0.55*math.Abs(s)
	ear := [2]float64{}
	for ch := range 2 {
		facing := (ch == 1) == (s >= 0)
		o.near[ch].set(o.rate, 0.25*behind)
		o.far[ch].set(o.rate, math.Abs(s)*0.9+0.25*behind)
		if facing {
			ear[ch] = o.near[ch].run(toward) * gainNear
		} else {
			ear[ch] = o.far[ch].run(away) * gainFar
		}
	}
	return ear[0] + side*0.35, ear[1] - side*0.35
}

// virtualizer plays left and right as from two speakers 30° to either side
// before the listener, and the sides of the sound as from further round,
// for headphones, which otherwise put it in the middle of the head.
type virtualizer struct {
	rate         float64
	l, r, s      line
	crossL       shadow // the left speaker as the right ear hears it
	crossR       shadow // and the right as the left
	rearL, rearR shadow
	cross, rear  float64 // the delays, in frames
	early        *reverb // the room's first echoes, that put it outside
}

// virtualSpeaker is the speakers' angle from straight ahead.
const virtualSpeaker = math.Pi / 6

func newVirtualizer(rate float64) *virtualizer {
	v := &virtualizer{rate: rate, l: newLine(int(rate*0.03) + 4), r: newLine(int(rate*0.03) + 4), s: newLine(int(rate*0.03) + 4)}
	v.cross = itd(virtualSpeaker) * rate
	v.rear = 0.012 * rate // the sides, from behind and apart
	v.crossL.set(rate, 0.55)
	v.crossR.set(rate, 0.55)
	v.rearL.set(rate, 0.7)
	v.rearR.set(rate, 0.7)
	v.early = newReverb(&Room{size: 0.35, damp: 0.6, wet: 0.14}, rate)
	return v
}

func (v *virtualizer) run(l, r float64) (float64, float64) {
	side := (l - r) / 2
	v.l.push(l)
	v.r.push(r)
	v.s.push(side)
	// Each ear hears its speaker, and the other's a moment later, duller
	// and softer, as round the head; and the sides from behind.
	el := l + 0.62*v.crossR.run(v.r.back(v.cross))
	er := r + 0.62*v.crossL.run(v.l.back(v.cross))
	rs := v.s.back(v.rear)
	el += 0.3 * v.rearL.run(rs)
	er -= 0.3 * v.rearR.run(v.s.back(v.rear*1.3))
	el, er = v.early.run(el, er)
	// As loud as the sound was: two speakers in each ear add up.
	return el * 0.62, er * 0.62
}
