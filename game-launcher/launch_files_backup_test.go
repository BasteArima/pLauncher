package main

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"game-launcher/internal/db"
	"game-launcher/internal/models"
)

func TestLaunchFilesBackupOldAndNew(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "legacy"}[legacy], func(t *testing.T) {
			a := newTestApp(t)
			g := &models.Game{ID: "files", Title: "Files", ExecPath: filepath.Join(t.TempDir(), "game.exe"), ExecLabel: "Primary", LaunchFiles: []models.LaunchFile{{Path: filepath.Join(t.TempDir(), "covers", "guide.pdf"), Label: "Walkthrough"}, {Path: filepath.Join(t.TempDir(), "config.exe")}}}
			if err := a.repo.SaveGame(a.ctx, g); err != nil {
				t.Fatal(err)
			}
			snap := filepath.Join(t.TempDir(), "snapshot.db")
			if err := a.repo.(*db.SQLiteRepo).BackupTo(a.ctx, snap); err != nil {
				t.Fatal(err)
			}
			if legacy {
				s, err := sql.Open("sqlite", snap)
				if err != nil {
					t.Fatal(err)
				}
				for _, col := range []string{"exec_label", "launch_files"} {
					if _, err := s.Exec(`ALTER TABLE games DROP COLUMN ` + col); err != nil {
						s.Close()
						t.Fatal(err)
					}
				}
				s.Close()
			}
			manifest, _ := json.Marshal(backupManifest{App: "pLauncher", Games: 1})
			archive := filepath.Join(t.TempDir(), "backup.zip")
			if err := writeBackupZip(archive, manifest, snap, a.dataDir); err != nil {
				t.Fatal(err)
			}
			dst := newTestApp(t)
			if n, err := dst.ImportLibrary(archive); err != nil || n != 1 {
				t.Fatal(n, err)
			}
			got, err := dst.gameByID(g.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.ExecPath != g.ExecPath {
				t.Fatal("primary rewritten", got)
			}
			if legacy {
				if got.ExecLabel != "" || len(got.LaunchFiles) != 0 {
					t.Fatal(got)
				}
			} else if got.ExecLabel != g.ExecLabel || !reflect.DeepEqual(got.LaunchFiles, g.LaunchFiles) {
				t.Fatal("launch references changed by import", got)
			}
		})
	}
}
