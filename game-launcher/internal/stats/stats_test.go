package stats

import (
	"fmt"
	"testing"
	"time"

	"game-launcher/internal/db"
	"game-launcher/internal/models"
)

func local(t *testing.T, zone, date string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatal(err)
	}
	v, err := time.ParseInLocation("2006-01-02 15:04", date, loc)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func ps(id, game string, start time.Time, seconds int64) models.PlaySession {
	return models.PlaySession{ID: id, GameID: game, StartedAt: start.Unix(), DurationSeconds: seconds}
}

func TestCalendarBoundariesAndDST(t *testing.T) {
	for _, tc := range []struct {
		name, zone, now, start string
		duration, week, month  int64
	}{
		{"week and month", "Europe/Moscow", "2026-06-01 02:00", "2026-05-31 23:30", 7200, 5400, 5400},
		{"year", "UTC", "2027-01-01 01:00", "2026-12-31 23:30", 3600, 3600, 1800},
		{"spring DST", "America/New_York", "2026-03-09 12:00", "2026-03-08 01:30", 7200, 0, 7200},
		{"fall DST", "America/New_York", "2026-11-02 12:00", "2026-11-01 00:30", 14400, 0, 14400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := local(t, tc.zone, tc.now)
			s := &db.SessionSnapshot{Games: []*models.Game{{ID: "g"}}, Sessions: []models.PlaySession{ps("s", "g", local(t, tc.zone, tc.start), tc.duration)}}
			out := Library(s, "month", now)
			if out.WeekSeconds != tc.week || out.MonthSeconds != tc.month {
				t.Fatalf("week/month: %d/%d", out.WeekSeconds, out.MonthSeconds)
			}
			var total int64
			for _, b := range out.Weeks {
				total += b.Seconds
			}
			if total != tc.duration {
				t.Fatalf("buckets lost time: %d", total)
			}
			if tc.name == "spring DST" && out.Weeks[50].End-out.Weeks[50].Start != 167*3600 {
				t.Fatal("spring week must have 167 hours")
			}
			if tc.name == "fall DST" && out.Weeks[50].End-out.Weeks[50].Start != 169*3600 {
				t.Fatal("fall week must have 169 hours")
			}
		})
	}
}

func TestOldTotalsCountsDiskAndTopGames(t *testing.T) {
	now := local(t, "UTC", "2026-11-06 12:00")
	s := &db.SessionSnapshot{HistorySince: 100, Games: []*models.Game{
		{ID: "old", Title: "Old", TimePlayed: 1000, Status: "finished", Engine: "Ren'Py", SizeBytes: 0, SizeCheckedAt: 10},
		{ID: "a", Title: "Alpha", TimePlayed: 2, Status: "playing", Engine: "Unity", SizeBytes: 1024, SizeCheckedAt: 10, CoverPath: "covers/a", Sources: []models.GameSource{{URL: "https://example.com"}}, UpdateAvailable: true},
		{ID: "b", Title: "Beta", TimePlayed: 2, Status: "weird", SizeBytes: 999, SizeCheckedAt: 10, FolderMissing: true},
		{ID: "c", Title: "Unknown size", SizeBytes: 0},
	}, Sessions: []models.PlaySession{ps("a", "a", now.Add(-time.Hour), 90), ps("b", "b", now.Add(-time.Hour), 90)}}
	month := Library(s, "month", now)
	all := Library(s, "all", now)
	if month.AllTimeSeconds != 1004*60 || month.MonthSeconds != 180 || month.WeekSeconds != 180 {
		t.Fatal(month)
	}
	if len(month.TopGames) != 2 || month.TopGames[0].GameID != "a" || all.TopGames[0].GameID != "old" {
		t.Fatal(month.TopGames, all.TopGames)
	}
	if month.Disk.Bytes != 1024 || month.Disk.Measured != 2 || month.Disk.Missing != 1 || month.Disk.Unknown != 1 {
		t.Fatal(month.Disk)
	}
	if month.Updates != 1 || month.NoCover != 3 || month.NoSource != 3 || month.Statuses[4].Count != 2 || month.Engines[0].Name != "" || month.Engines[0].Count != 2 {
		t.Fatal(month)
	}
	old := Game(s, "old", now)
	if old.Count != 0 || len(old.Latest) != 0 || len(old.Weeks) != 8 {
		t.Fatal(old)
	}
}

func TestHistoryEightWeeksLatestFiveAndRankingLimit(t *testing.T) {
	now := local(t, "UTC", "2026-11-06 12:00")
	s := &db.SessionSnapshot{}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("%02d", i)
		s.Games = append(s.Games, &models.Game{ID: id, Title: "Tie", TimePlayed: 10})
		s.Sessions = append(s.Sessions, ps(id, "00", now.Add(-time.Duration(i)*24*time.Hour), int64(60+i)))
	}
	out := Game(s, "00", now)
	if len(out.Weeks) != 8 || len(out.Latest) != 5 || out.Latest[0].ID != "00" || out.Count != 12 || out.LongestSeconds != 71 {
		t.Fatal(out)
	}
	all := Library(s, "all", now)
	if len(all.Weeks) != 52 || len(all.TopGames) != 10 || all.TopGames[0].GameID != "00" || all.TopGames[9].GameID != "09" {
		t.Fatal(all)
	}
	for i := 1; i < len(all.Weeks); i++ {
		if all.Weeks[i-1].End != all.Weeks[i].Start {
			t.Fatal("gap in weeks")
		}
	}
	empty := Library(&db.SessionSnapshot{}, "month", now)
	if len(empty.Weeks) != 52 || len(empty.TopGames) != 0 || empty.AllTimeSeconds != 0 {
		t.Fatal(empty)
	}
}

func TestTimezoneAndFutureClipping(t *testing.T) {
	now := local(t, "UTC", "2026-06-01 00:30")
	s := &db.SessionSnapshot{Games: []*models.Game{{ID: "g"}}, Sessions: []models.PlaySession{ps("s", "g", now.Add(-time.Hour), 7200)}}
	utc := Library(s, "month", now)
	ny, _ := time.LoadLocation("America/New_York")
	other := Library(s, "month", now.In(ny))
	if utc.MonthSeconds != 1800 || other.MonthSeconds != 3600 {
		t.Fatal(utc.MonthSeconds, other.MonthSeconds)
	}
	if s.Sessions[0].DurationSeconds != 7200 {
		t.Fatal("regrouping mutated session")
	}
}
