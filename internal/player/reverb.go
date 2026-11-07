package player

// The rooms the sound may play in, as Freeverb (Jezar at Dreampoint,
// public domain) makes them: eight combs that ring as a room's walls do,
// their highs dying sooner, then four all-passes that blur their echoes
// into one; the right channel's a little longer than the left's, for a
// room around the listener.

// Room is one of the rooms the effects may play the sound in.
type Room struct {
	ID, Label string
	size      float64 // how long it rings, 0 to 1
	damp      float64 // how soon its highs die, 0 to 1
	wet       float64 // how much of it is heard against the sound
}

// Rooms are the rooms the effects offer, the first none.
var Rooms = []Room{
	{"", "无", 0, 0, 0},
	{"bathroom", "浴室", 0.42, 0.08, 0.32},
	{"church", "教堂", 0.95, 0.25, 0.38},
	{"concert", "演唱会", 0.86, 0.35, 0.34},
	{"hall", "大厅", 0.78, 0.45, 0.28},
	{"room", "房间", 0.5, 0.55, 0.2},
}

// roomOf is the room of id, nil for none.
func roomOf(id string) *Room {
	for i := range Rooms {
		if Rooms[i].ID == id && id != "" {
			return &Rooms[i]
		}
	}
	return nil
}

// Freeverb's lengths, in frames at 44.1 kHz, and its stereo spread.
var (
	combTuning    = [8]int{1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617}
	allpassTuning = [4]int{556, 441, 341, 225}
)

const stereoSpread = 23

type comb struct {
	buf      []float64
	at       int
	store    float64
	feedback float64
	damp     float64
}

func (c *comb) run(x float64) float64 {
	y := c.buf[c.at]
	c.store = y*(1-c.damp) + c.store*c.damp
	c.buf[c.at] = x + c.store*c.feedback
	if c.at++; c.at == len(c.buf) {
		c.at = 0
	}
	return y
}

type allpass struct {
	buf []float64
	at  int
}

func (a *allpass) run(x float64) float64 {
	b := a.buf[a.at]
	a.buf[a.at] = x + b*0.5
	if a.at++; a.at == len(a.buf) {
		a.at = 0
	}
	return b - x
}

// reverb is a room for both channels.
type reverb struct {
	combs [2][8]comb
	all   [2][4]allpass
	wet   float64
}

func newReverb(r *Room, rate float64) *reverb {
	scale := rate / 44100
	v := &reverb{wet: r.wet}
	for ch := range 2 {
		spread := ch * stereoSpread
		for i, n := range combTuning {
			v.combs[ch][i] = comb{buf: make([]float64, int(float64(n+spread)*scale)+1),
				feedback: r.size*0.28 + 0.7, damp: r.damp * 0.4}
		}
		for i, n := range allpassTuning {
			v.all[ch][i] = allpass{buf: make([]float64, int(float64(n+spread)*scale)+1)}
		}
	}
	return v
}

// run adds the room's echoes of l and r to them.
func (v *reverb) run(l, r float64) (float64, float64) {
	in := (l + r) * 0.015
	var out [2]float64
	for ch := range 2 {
		var sum float64
		for i := range v.combs[ch] {
			sum += v.combs[ch][i].run(in)
		}
		for i := range v.all[ch] {
			sum = v.all[ch][i].run(sum)
		}
		out[ch] = sum
	}
	dry := 1 - v.wet*0.4
	return l*dry + out[0]*v.wet*3, r*dry + out[1]*v.wet*3
}
