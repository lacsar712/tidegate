package app

import (
	"time"

	"github.com/lacsar712/tidegate/internal/model"
)

// RegisterChamber adds a chamber definition tracked by the app snapshot.
func (a *App) RegisterChamber(ch model.Chamber) {
	a.chamberMu.Lock()
	defer a.chamberMu.Unlock()
	if ch.UpdatedAt.IsZero() {
		ch.UpdatedAt = a.now().UTC()
	}
	a.chambers[ch.ID] = ch
	a.levels.EnsureChamber(ch.ID)
}

// Chamber returns a cloned chamber snapshot by id.
func (a *App) Chamber(id string) (model.Chamber, bool) {
	a.chamberMu.RLock()
	defer a.chamberMu.RUnlock()
	ch, ok := a.chambers[id]
	return ch.Clone(), ok
}

// SeedDemoLevels inserts balanced demo readings for UI smoke tests.
func (a *App) SeedDemoLevels(chamberID string, upstream, downstream int) error {
	now := a.now().UTC()
	a.levels.EnsureChamber(chamberID)
	for i := 0; i < 5; i++ {
		at := now.Add(time.Duration(i) * time.Second)
		_, _, _, _ = a.levels.Ingest(model.LevelReport{
			ChamberID:  chamberID,
			Probe:      model.ProbeUpstream,
			LevelCM:    upstream + i%2,
			ReportedAt: at,
		}, at)
		up, down, head, err := a.levels.Ingest(model.LevelReport{
			ChamberID:  chamberID,
			Probe:      model.ProbeDownstream,
			LevelCM:    downstream,
			ReportedAt: at,
		}, at)
		if err != nil {
			return err
		}
		if i == 4 {
			a.chamberMu.Lock()
			if ch, ok := a.chambers[chamberID]; ok {
				ch.ApplySmoothedLevels(up, down, head, at)
				a.chambers[chamberID] = ch
			}
			a.chamberMu.Unlock()
		}
	}
	return nil
}

// Config returns the active configuration snapshot.
func (a *App) Config() configSnapshot {
	return configSnapshot{
		HeadDiffLimitCM: a.cfg.HeadDiffLimitCM,
		TicketTTL:       a.cfg.TicketTTL,
		BypassEnabled:   a.cfg.BypassEnabled,
	}
}

type configSnapshot struct {
	HeadDiffLimitCM int
	TicketTTL       time.Duration
	BypassEnabled   bool
}
