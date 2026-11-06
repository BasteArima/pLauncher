package main

import (
	"context"
	"errors"
	"time"

	"game-launcher/internal/apperr"
	"game-launcher/internal/db"
	"game-launcher/internal/launcher"
	"game-launcher/internal/models"
	"game-launcher/internal/sessionlock"
	"game-launcher/internal/stats"
)

func sessionErr(err error) error {
	if err == nil {
		return nil
	}
	return apperr.New("sessions.storage", nil, "couldn't read or save play sessions: %w", err)
}

func sessionsBusy() error {
	return apperr.New("sessions.busy", nil, "wait for running games and session recording to finish before changing data or installing an update")
}

// Guard includes both paths for a move. It is held until the operation commits;
// Launch uses the same mutex and OS gate, including its monitor startup handshake.
func (a *App) guardSessionData(targets ...string) (func(), error) {
	a.sessionsMu.Lock()
	locks := []*sessionlock.Lock{}
	release := func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Close()
		}
		a.sessionsMu.Unlock()
	}
	seen := map[string]bool{}
	for _, dir := range append([]string{a.dataDir}, targets...) {
		if dir == "" {
			continue
		}
		key, err := sessionlock.Dir(dir)
		if err != nil {
			release()
			return nil, sessionErr(err)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		l, err := sessionlock.Try(dir, "gate")
		if err != nil {
			release()
			if errors.Is(err, sessionlock.ErrBusy) {
				return nil, sessionsBusy()
			}
			return nil, sessionErr(err)
		}
		locks = append(locks, l)
		active, err := sessionlock.Active(dir)
		if err != nil {
			release()
			return nil, sessionErr(err)
		}
		if active {
			release()
			return nil, sessionsBusy()
		}
	}
	return release, nil
}

func (a *App) recoverSessions() error {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	if a.dataDir == "" {
		return nil
	}
	l, err := sessionlock.Try(a.dataDir, "gate")
	if err != nil {
		return sessionErr(err)
	}
	defer l.Close()
	a.mu.Lock()
	defer a.mu.Unlock()
	if repo, ok := a.repo.(*db.SQLiteRepo); ok {
		return sessionErr(launcher.Recover(context.Background(), repo, a.dataDir))
	}
	return nil
}

func (a *App) launchTracked(gameID, exe, folder string) error {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	return a.launchTrackedLocked(gameID, exe, folder)
}

func (a *App) launchTrackedLocked(gameID, exe, folder string) error {
	if a.repo == nil || a.isLocked() {
		return nil
	}
	if !launcher.CanTrack(exe) {
		l, err := sessionlock.Try(a.dataDir, "gate")
		if err != nil {
			return sessionErr(err)
		}
		defer l.Close()
		if err := a.launcher.LaunchAssociated(exe, folder); err != nil {
			return apperr.New("sessions.launch", nil, "couldn't launch game: %w", err)
		}
		return sessionErr(a.repo.MarkLaunched(a.ctx, gameID, time.Now().Unix()))
	}
	repo, ok := a.repo.(*db.SQLiteRepo)
	if !ok {
		return sessionErr(errors.New("session repository unavailable"))
	}
	if err := launcher.StartMonitored(a.ctx, repo, a.dataDir, gameID, exe, folder); err != nil {
		return apperr.New("sessions.launch", nil, "couldn't launch game: %w", err)
	}
	return nil
}

func statsLocation(zone string) (*time.Location, error) {
	if zone == "" || zone == "Local" {
		return nil, apperr.New("sessions.timezone", nil, "select a valid IANA time zone")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, apperr.New("sessions.timezone", nil, "select a valid IANA time zone")
	}
	return loc, nil
}

func (a *App) GetGameSessions(gameID string, visibleGameIDs []string, timeZone string) (*models.GameSessions, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.repo == nil || a.isLocked() {
		return nil, nil
	}
	allowed := false
	for _, id := range visibleGameIDs {
		if id == gameID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, nil
	}
	loc, err := statsLocation(timeZone)
	if err != nil {
		return nil, err
	}
	snapshot, err := a.repo.ReadSessionSnapshot(a.ctx, []string{gameID})
	if a.isLocked() {
		return nil, nil
	}
	if err != nil {
		return nil, sessionErr(err)
	}
	if len(snapshot.Games) == 0 {
		return nil, nil
	}
	result := stats.Game(snapshot, gameID, time.Now().In(loc))
	result.TrackingSupported = launcher.CanTrack(snapshot.Games[0].ExecPath)
	if a.isLocked() {
		return nil, nil
	}
	return result, nil
}

func (a *App) GetLibraryStats(visibleGameIDs []string, period, timeZone string) (*models.LibraryStats, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.repo == nil || a.isLocked() {
		return nil, nil
	}
	if period != "all" && period != "month" && period != "week" {
		return nil, apperr.New("sessions.period", nil, "select a valid statistics period")
	}
	loc, err := statsLocation(timeZone)
	if err != nil {
		return nil, err
	}
	snapshot, err := a.repo.ReadSessionSnapshot(a.ctx, visibleGameIDs)
	if a.isLocked() {
		return nil, nil
	}
	if err != nil {
		return nil, sessionErr(err)
	}
	for _, g := range snapshot.Games {
		g.FolderMissing = folderMissing(g.FolderPath)
	}
	result := stats.Library(snapshot, period, time.Now().In(loc))
	if a.isLocked() {
		return nil, nil
	}
	return result, nil
}

// Nil when locked/unconfigured, including no revision (which could disclose activity).
func (a *App) GetSessionRevision() (*int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.repo == nil || a.isLocked() {
		return nil, nil
	}
	rev, message, err := a.repo.SessionRevision(a.ctx)
	if a.isLocked() {
		return nil, nil
	}
	if err != nil {
		return nil, sessionErr(err)
	}
	if message != "" {
		return nil, sessionErr(errors.New(message))
	}
	return &rev, nil
}

func (a *App) watchSessions() {
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.sessionsStop = cancel
	a.mu.Unlock()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		var previous int64 = -1
		var previousError string
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Recover monitors that died while this UI stayed open, too. Recording
				// and recovery continue behind the privacy lock; reads/events do not.
				recoveryErr := a.recoverSessions()
				if a.isLocked() {
					previous = -1
					previousError = ""
					continue
				}
				rev, err := a.GetSessionRevision()
				if err == nil {
					err = recoveryErr
				}
				if err != nil {
					if err.Error() != previousError && !a.isLocked() {
						a.emit("session-recording-error", err.Error())
						previousError = err.Error()
					}
					continue
				}
				previousError = ""
				if rev != nil && *rev != previous {
					previous = *rev
					if !a.isLocked() {
						a.emit("games-updated")
						a.emit("sessions-updated")
					}
				}
			}
		}
	}()
}
