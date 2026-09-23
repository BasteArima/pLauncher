package main

import (
	"context"
	"strings"
	"testing"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	a := &App{ctx: context.Background()}
	if err := a.initServices(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.shutdown(a.ctx) })
	return a
}

func TestPinLifecycle(t *testing.T) {
	a := newTestApp(t)

	if err := a.Lock(); err == nil {
		t.Fatal("без PIN блокировка невозможна")
	}
	if err := a.SetPin("", "12"); err == nil {
		t.Fatal("слишком короткий PIN должен отклоняться")
	}
	if err := a.SetPin("", "1234"); err != nil {
		t.Fatal(err)
	}
	if !a.GetPrivacy().HasPin {
		t.Fatal("has_pin = false после установки")
	}
	// Хэш хранится, сам PIN — нет
	raw, _ := a.repo.GetSetting(a.ctx, privacyKey)
	if strings.Contains(raw, "1234") {
		t.Fatal("PIN хранится в открытом виде")
	}

	if err := a.Lock(); err != nil {
		t.Fatal(err)
	}
	if games, _ := a.GetGames(); len(games) != 0 || !a.GetPrivacy().Locked {
		t.Fatal("в заблокированном состоянии игры отдаваться не должны")
	}
	if err := a.Unlock("0000"); err == nil {
		t.Fatal("неверный PIN принят")
	}
	if err := a.Unlock("1234"); err != nil || a.isLocked() {
		t.Fatalf("разблокировка: %v", err)
	}

	// Смена PIN требует старый; пустой новый — снимает PIN
	if err := a.SetPin("wrong", "5678"); err == nil {
		t.Fatal("смена PIN без верного старого")
	}
	if err := a.SetPin("1234", ""); err != nil || a.GetPrivacy().HasPin {
		t.Fatalf("снятие PIN: %v", err)
	}
}

func TestPinBruteForceCooldown(t *testing.T) {
	a := newTestApp(t)
	a.SetPin("", "1234")
	for i := 0; i < pinMaxAttempts; i++ {
		a.VerifyPin("0000")
	}
	if err := a.VerifyPin("1234"); err == nil || !strings.Contains(err.Error(), "wait") {
		t.Fatalf("после %d ошибок ожидалась пауза, got %v", pinMaxAttempts, err)
	}
}

func TestSavePrivacyKeepsPin(t *testing.T) {
	a := newTestApp(t)
	a.SetPin("", "1234")
	s := a.GetPrivacy()
	s.BlurMode = "hover"
	s.IdleLockMin = 15
	s.PanicEnabled = false
	if err := a.SavePrivacy(s); err != nil {
		t.Fatal(err)
	}
	got := a.GetPrivacy()
	if got.BlurMode != "hover" || got.IdleLockMin != 15 || !got.HasPin {
		t.Fatalf("настройки не сохранились или потерялся PIN: %+v", got)
	}
	s.BlurMode = "bogus"
	a.SavePrivacy(s)
	if a.GetPrivacy().BlurMode != "none" {
		t.Error("неизвестный режим размытия должен сбрасываться в none")
	}
}
