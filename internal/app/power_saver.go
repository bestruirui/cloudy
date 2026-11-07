package app

// savedEffects are the effects that cost power, as they were before the
// power saver turned them off.
type savedEffects struct {
	LiquidGlass bool `json:"liquidGlass"`
	BarSpin     bool `json:"barSpin"`
	NpLight     bool `json:"npLight"`
	Fog         bool `json:"fog"`
	Smoke       bool `json:"smoke"`
}

// setPowerSaver turns the effects that cost power off, keeping how they
// were, or puts them back as they were.
func (a *app) setPowerSaver(on bool) {
	s := &a.settings
	if on == s.PowerSaver {
		return
	}
	if on {
		s.SaverBefore = &savedEffects{s.LiquidGlass, s.BarSpin, s.NpLight, s.Fog, s.Smoke}
		s.LiquidGlass, s.BarSpin, s.NpLight, s.Fog, s.Smoke = false, false, false, false, false
	} else if b := s.SaverBefore; b != nil {
		s.LiquidGlass, s.BarSpin, s.NpLight, s.Fog, s.Smoke = b.LiquidGlass, b.BarSpin, b.NpLight, b.Fog, b.Smoke
	}
	s.PowerSaver = on
	if !on {
		s.SaverBefore = nil
	}
	a.changed()
}

// leavePowerSaver ends the saver as the listener turns an effect back on by
// hand, leaving the others as they are now.
func (a *app) leavePowerSaver() {
	s := &a.settings
	if !s.PowerSaver {
		return
	}
	if s.LiquidGlass || s.BarSpin || s.NpLight || s.Fog || s.Smoke {
		s.PowerSaver, s.SaverBefore = false, nil
	}
}
