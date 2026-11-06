package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"game-launcher/internal/models"
)

// PendingSession существует до подтверждённой финализации; ID также служит токеном монитора.
type PendingSession struct {
	ID, GameID, Executable, Folder string
	StartedAt, Seconds             int64
	Complete                       bool
	Error                          string
}

type SessionSnapshot struct {
	Games        []*models.Game
	Sessions     []models.PlaySession
	HistorySince int64
}

func (r *SQLiteRepo) initSessions() error {
	_, err := r.db.Exec(`
CREATE TABLE IF NOT EXISTS play_sessions (
 id TEXT PRIMARY KEY, game_id TEXT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
 started_at INTEGER NOT NULL, duration_seconds INTEGER NOT NULL CHECK(duration_seconds >= 60),
 interrupted INTEGER NOT NULL DEFAULT 0);
CREATE INDEX IF NOT EXISTS sessions_game_start ON play_sessions(game_id, started_at);
CREATE INDEX IF NOT EXISTS sessions_start ON play_sessions(started_at);
CREATE TABLE IF NOT EXISTS pending_sessions (
 id TEXT PRIMARY KEY, game_id TEXT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
 executable TEXT NOT NULL, folder TEXT NOT NULL, started_at INTEGER NOT NULL DEFAULT 0,
 seconds INTEGER NOT NULL DEFAULT 0, complete INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT '');
INSERT OR IGNORE INTO settings(key,value) VALUES('sessions_revision','0');
CREATE TRIGGER IF NOT EXISTS sessions_insert_revision AFTER INSERT ON play_sessions BEGIN
 UPDATE settings SET value=CAST(value AS INTEGER)+1 WHERE key='sessions_revision'; END;
CREATE TRIGGER IF NOT EXISTS sessions_delete_revision AFTER DELETE ON play_sessions BEGIN
 UPDATE settings SET value=CAST(value AS INTEGER)+1 WHERE key='sessions_revision'; END;
CREATE TRIGGER IF NOT EXISTS sessions_move_revision AFTER UPDATE ON play_sessions BEGIN
 UPDATE settings SET value=CAST(value AS INTEGER)+1 WHERE key='sessions_revision'; END;
`)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`INSERT OR IGNORE INTO settings(key,value) VALUES('sessions_since',?)`, time.Now().Unix())
	return err
}

func (r *SQLiteRepo) BeginSession(ctx context.Context, p PendingSession) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO pending_sessions(id,game_id,executable,folder) VALUES(?,?,?,?)`, p.ID, p.GameID, p.Executable, p.Folder)
	return err
}

func (r *SQLiteRepo) StartSession(ctx context.Context, id string, at int64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE pending_sessions SET started_at=? WHERE id=? AND started_at=0`, at, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE games SET last_launched_at=MAX(COALESCE(last_launched_at,0),?) WHERE id=(SELECT game_id FROM pending_sessions WHERE id=?)`, at, id)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *SQLiteRepo) MarkLaunched(ctx context.Context, id string, at int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE games SET last_launched_at=MAX(COALESCE(last_launched_at,0),?) WHERE id=?`, at, id)
	return err
}

func (r *SQLiteRepo) CheckpointSession(ctx context.Context, id string, seconds int64, complete bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE pending_sessions SET seconds=CASE WHEN complete=1 THEN seconds WHEN ? THEN ? ELSE MAX(seconds,?) END,complete=MAX(complete,?),error='' WHERE id=?`, complete, max(0, seconds), max(0, seconds), complete, id)
	return err
}

func (r *SQLiteRepo) SessionError(ctx context.Context, id string, message string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE pending_sessions SET error=? WHERE id=?`, message, id)
	return err
}

func (r *SQLiteRepo) CancelSession(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM pending_sessions WHERE id=?`, id)
	return err
}

func (r *SQLiteRepo) PendingSessions(ctx context.Context) ([]PendingSession, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,game_id,executable,folder,started_at,seconds,complete,error FROM pending_sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PendingSession{}
	for rows.Next() {
		var p PendingSession
		if err := rows.Scan(&p.ID, &p.GameID, &p.Executable, &p.Folder, &p.StartedAt, &p.Seconds, &p.Complete, &p.Error); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DELETE RETURNING получает блокировку записи до чтения: два монитора/повторный вызов
// не могут начислить время дважды. Ошибка на любом этапе откатывает и pending, и минуты.
func (r *SQLiteRepo) FinishSession(ctx context.Context, id string, interrupted bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var p PendingSession
	err = tx.QueryRowContext(ctx, `DELETE FROM pending_sessions WHERE id=? RETURNING game_id,started_at,seconds,complete`, id).Scan(&p.GameID, &p.StartedAt, &p.Seconds, &p.Complete)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if p.StartedAt > 0 && p.Seconds >= 60 {
		res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO play_sessions(id,game_id,started_at,duration_seconds,interrupted) VALUES(?,?,?,?,?)`, id, p.GameID, p.StartedAt, p.Seconds, interrupted && !p.Complete)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			if _, err = tx.ExecContext(ctx, `UPDATE games SET time_played=COALESCE(time_played,0)+? WHERE id=?`, p.Seconds/60, p.GameID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// RecoverImportedSessions never attaches to processes from the source computer.
func (r *SQLiteRepo) RecoverImportedSessions(ctx context.Context) error {
	ps, err := r.PendingSessions(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if err := r.FinishSession(ctx, p.ID, true); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepo) SessionRevision(ctx context.Context) (int64, string, error) {
	var rev int64
	err := r.db.QueryRowContext(ctx, `SELECT CAST(value AS INTEGER) FROM settings WHERE key='sessions_revision'`).Scan(&rev)
	if err != nil {
		return 0, "", err
	}
	var message string
	err = r.db.QueryRowContext(ctx, `SELECT error FROM pending_sessions WHERE error<>'' LIMIT 1`).Scan(&message)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return rev, message, err
}

// Один snapshot для всех агрегатов; JSON-параметр не ограничивает библиотеку числом bind variables.
func (r *SQLiteRepo) ReadSessionSnapshot(ctx context.Context, ids []string) (*SessionSnapshot, error) {
	if ids == nil {
		ids = []string{}
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	s := &SessionSnapshot{}
	err = tx.QueryRowContext(ctx, `SELECT CAST(value AS INTEGER) FROM settings WHERE key='sessions_since'`).Scan(&s.HistorySince)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+gameColumns+` FROM games WHERE id IN (SELECT value FROM json_each(?))`, string(raw))
	if err != nil {
		return nil, err
	}
	s.Games, err = scanGames(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,game_id,started_at,duration_seconds,interrupted FROM play_sessions WHERE game_id IN (SELECT value FROM json_each(?)) ORDER BY started_at DESC,id`, string(raw))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p models.PlaySession
		if err := rows.Scan(&p.ID, &p.GameID, &p.StartedAt, &p.DurationSeconds, &p.Interrupted); err != nil {
			return nil, err
		}
		s.Sessions = append(s.Sessions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return s, tx.Commit()
}

// SaveRelink moves sessions and totals before deleting the duplicate, in the same transaction.
func (r *SQLiteRepo) SaveRelink(ctx context.Context, g *models.Game, duplicateID string) error {
	if duplicateID == g.ID {
		return fmt.Errorf("cannot merge a game into itself")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// First statement acquires the writer lock. Keep the latest time even if a monitor just exited.
	res, err := tx.ExecContext(ctx, `UPDATE games SET time_played=COALESCE(time_played,0)+COALESCE((SELECT time_played FROM games WHERE id=?),0), last_launched_at=MAX(COALESCE(last_launched_at,0),COALESCE((SELECT last_launched_at FROM games WHERE id=?),0)) WHERE id=?`, duplicateID, duplicateID, g.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("relink target not found")
	}
	// Перенос заметки не является правкой: сохраняем исходную дату до общего UPSERT.
	if duplicateID != "" {
		_, err := tx.ExecContext(ctx, `UPDATE games SET notes=(SELECT notes FROM games WHERE id=?), notes_updated_at=COALESCE((SELECT notes_updated_at FROM games WHERE id=?),0) WHERE id=? AND COALESCE(notes,'')='' AND EXISTS(SELECT 1 FROM games WHERE id=? AND COALESCE(notes,'')!='')`, duplicateID, duplicateID, g.ID, duplicateID)
		if err != nil {
			return err
		}
	}
	if err := saveGame(ctx, tx, g); err != nil {
		return err
	}
	if duplicateID != "" && duplicateID != g.ID {
		for _, table := range []string{"play_sessions", "pending_sessions"} {
			if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET game_id=? WHERE game_id=?`, g.ID, duplicateID); err != nil {
				return err
			}
		}
		var raw string
		err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='collections'`).Scan(&raw)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if raw != "" {
			var cols []models.Collection
			if err := json.Unmarshal([]byte(raw), &cols); err != nil {
				return err
			}
			for i := range cols {
				seen := map[string]bool{}
				ids := []string{}
				for _, id := range cols[i].GameIDs {
					if id == duplicateID {
						id = g.ID
					}
					if !seen[id] {
						ids = append(ids, id)
						seen[id] = true
					}
				}
				cols[i].GameIDs = ids
			}
			b, err := json.Marshal(cols)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE settings SET value=? WHERE key='collections'`, string(b)); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM games WHERE id=?`, duplicateID); err != nil {
			return err
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT time_played,last_launched_at FROM games WHERE id=?`, g.ID).Scan(&g.TimePlayed, &g.LastLaunchedAt); err != nil {
		return err
	}
	return tx.Commit()
}
