// Package stats aggregates recorded intervals. Storage timestamps are UTC; calendar
// boundaries use the caller's IANA location (including DST, never fixed 24-hour days).
package stats

import (
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"game-launcher/internal/db"
	"game-launcher/internal/models"
)

func weekStart(now time.Time) time.Time {
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return d.AddDate(0, 0, -(int(d.Weekday())+6)%7)
}

func weeks(now time.Time, n int) []models.WeekActivity {
	start := weekStart(now).AddDate(0, 0, -7*(n-1))
	out := make([]models.WeekActivity, n)
	for i := range out {
		end := start.AddDate(0, 0, 7)
		out[i] = models.WeekActivity{Start: start.Unix(), End: end.Unix()}
		start = end
	}
	return out
}

func overlap(s models.PlaySession, from, to int64) int64 {
	// Durations originate from the monotonic monitor, but reject malformed imported intervals.
	if s.DurationSeconds < 60 || s.StartedAt < 0 || s.DurationSeconds > 1<<62-s.StartedAt {
		return 0
	}
	return max(0, min(s.StartedAt+s.DurationSeconds, to)-max(s.StartedAt, from))
}

func addWeeks(out []models.WeekActivity, s models.PlaySession, now int64) {
	for i := range out {
		out[i].Seconds += overlap(s, out[i].Start, min(out[i].End, now))
	}
}

func Game(snapshot *db.SessionSnapshot, id string, now time.Time) *models.GameSessions {
	out := &models.GameSessions{HistorySince: snapshot.HistorySince, Weeks: weeks(now, 8), Latest: []models.PlaySession{}}
	sessions := []models.PlaySession{}
	for _, s := range snapshot.Sessions {
		if s.GameID != id {
			continue
		}
		out.Count++
		out.LongestSeconds = max(out.LongestSeconds, s.DurationSeconds)
		addWeeks(out.Weeks, s, now.Unix())
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].StartedAt != sessions[j].StartedAt {
			return sessions[i].StartedAt > sessions[j].StartedAt
		}
		return sessions[i].ID < sessions[j].ID
	})
	if len(sessions) > 5 {
		sessions = sessions[:5]
	}
	out.Latest = sessions
	return out
}

func Library(snapshot *db.SessionSnapshot, period string, now time.Time) *models.LibraryStats {
	out := &models.LibraryStats{HistorySince: snapshot.HistorySince, Weeks: weeks(now, 52), TopGames: []models.GameTime{}, Engines: []models.NamedCount{}}
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	week := weekStart(now).Unix()
	times := map[string]int64{}
	titles := map[string]string{}
	engines := map[string]int{}
	statuses := map[string]int{}
	for _, g := range snapshot.Games {
		titles[g.ID] = g.Title
		total := max(0, int64(g.TimePlayed)) * 60
		out.AllTimeSeconds += total
		if period == "all" {
			times[g.ID] = total
		}
		status := g.Status
		switch status {
		case "playing", "want", "finished", "dropped":
		default:
			status = ""
		}
		statuses[status]++
		engines[strings.TrimSpace(g.Engine)]++
		if g.FolderMissing {
			out.Disk.Missing++
		} else if g.SizeCheckedAt <= 0 {
			out.Disk.Unknown++
		} else {
			out.Disk.Measured++
			out.Disk.Bytes += max(0, g.SizeBytes)
		}
		if g.UpdateAvailable {
			out.Updates++
		}
		if strings.TrimSpace(g.CoverPath) == "" {
			out.NoCover++
		}
		hasSource := false
		for _, s := range g.Sources {
			if strings.TrimSpace(s.URL) != "" {
				hasSource = true
				break
			}
		}
		if !hasSource {
			out.NoSource++
		}
	}
	for _, s := range snapshot.Sessions {
		if _, ok := titles[s.GameID]; !ok {
			continue
		}
		m, w := overlap(s, month, now.Unix()), overlap(s, week, now.Unix())
		out.MonthSeconds += m
		out.WeekSeconds += w
		if period == "month" {
			times[s.GameID] += m
		} else if period == "week" {
			times[s.GameID] += w
		}
		addWeeks(out.Weeks, s, now.Unix())
	}
	for id, seconds := range times {
		if seconds > 0 {
			out.TopGames = append(out.TopGames, models.GameTime{GameID: id, Seconds: seconds})
		}
	}
	sort.Slice(out.TopGames, func(i, j int) bool {
		a, b := out.TopGames[i], out.TopGames[j]
		if a.Seconds != b.Seconds {
			return a.Seconds > b.Seconds
		}
		if titles[a.GameID] != titles[b.GameID] {
			return titles[a.GameID] < titles[b.GameID]
		}
		return a.GameID < b.GameID
	})
	if len(out.TopGames) > 10 {
		out.TopGames = out.TopGames[:10]
	}
	for _, name := range []string{"playing", "want", "finished", "dropped", ""} {
		out.Statuses = append(out.Statuses, models.NamedCount{Name: name, Count: statuses[name]})
	}
	for name, count := range engines {
		out.Engines = append(out.Engines, models.NamedCount{Name: name, Count: count})
	}
	sort.Slice(out.Engines, func(i, j int) bool {
		a, b := out.Engines[i], out.Engines[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Name < b.Name
	})
	return out
}
