package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// =========================================================================
// ПРИВАТНОСТЬ: PIN-блокировка, кнопка паники (глобальная горячая клавиша),
// размытие обложек. Настройки лежат в БД (settings["privacy"]), т.е. переезжают
// вместе с библиотекой. PIN закрывает интерфейс и отдачу данных, но файлы не шифрует.
// =========================================================================

// PrivacySettings — то, что видит и меняет фронтенд (без хэша PIN).
type PrivacySettings struct {
	HasPin        bool   `json:"has_pin"`
	Locked        bool   `json:"locked"`
	IdleLockMin   int    `json:"idle_lock_min"`  // автоблокировка после N минут бездействия (0 — выкл)
	PanicEnabled  bool   `json:"panic_enabled"`  // глобальная горячая клавиша включена
	PanicMods     uint32 `json:"panic_mods"`     // MOD_ALT=1, MOD_CONTROL=2, MOD_SHIFT=4, MOD_WIN=8
	PanicVK       uint32 `json:"panic_vk"`       // virtual-key code
	PanicLabel    string `json:"panic_label"`    // подпись для UI, напр. "Ctrl+Alt+H"
	PanicAction   string `json:"panic_action"`   // hide | minimize
	LockOnPanic   bool   `json:"lock_on_panic"`  // при панике блокировать PIN-кодом
	BlurMode      string `json:"blur_mode"`      // none | hover | always
	StartDiscreet bool   `json:"start_discreet"` // запускаться в дискретном режиме (Ctrl+H)
}

type privacyStored struct {
	PrivacySettings
	PinHash string `json:"pin_hash,omitempty"`
	PinSalt string `json:"pin_salt,omitempty"`
	PinIter int    `json:"pin_iter,omitempty"`
}

const (
	privacyKey     = "privacy"
	pinIterations  = 200_000
	pinMinLength   = 4
	pinMaxAttempts = 5
	pinCooldown    = 30 * time.Second
)

// privacyState — состояние сессии (не хранится).
type privacyState struct {
	mu          sync.Mutex
	locked      bool
	panicHidden bool // окно спрятано кнопкой паники
	fails       int
	lastFail    time.Time
	hotkeys     hotkeyManager
}

func defaultPrivacy() privacyStored {
	return privacyStored{PrivacySettings: PrivacySettings{
		PanicMods:   1 | 2, // Ctrl+Alt
		PanicVK:     'H',
		PanicLabel:  "Ctrl+Alt+H",
		PanicAction: "hide",
		LockOnPanic: true,
		BlurMode:    "none",
	}}
}

func (a *App) loadPrivacy() privacyStored {
	p := defaultPrivacy()
	if a.repo == nil {
		return p
	}
	if raw, _ := a.repo.GetSetting(a.ctx, privacyKey); raw != "" {
		json.Unmarshal([]byte(raw), &p)
	}
	p.HasPin = p.PinHash != ""
	return p
}

func (a *App) savePrivacy(p privacyStored) error {
	if a.repo == nil {
		return errNoDataDir()
	}
	p.HasPin, p.Locked = p.PinHash != "", false // вычисляемые поля не храним
	data, _ := json.Marshal(p)
	return a.repo.SetSetting(a.ctx, privacyKey, string(data))
}

func (a *App) isLocked() bool {
	a.priv.mu.Lock()
	defer a.priv.mu.Unlock()
	return a.priv.locked
}

// GetPrivacy возвращает настройки приватности и текущее состояние блокировки.
func (a *App) GetPrivacy() PrivacySettings {
	p := a.loadPrivacy().PrivacySettings
	p.Locked = a.isLocked()
	return p
}

// SavePrivacy сохраняет настройки (кроме PIN). Если новую горячую клавишу не удалось
// зарегистрировать, она сохраняется выключенной, а ошибка возвращается в UI.
func (a *App) SavePrivacy(s PrivacySettings) error {
	if a.isLocked() {
		return fmt.Errorf("launcher is locked")
	}
	p := a.loadPrivacy()
	if s.BlurMode != "hover" && s.BlurMode != "always" {
		s.BlurMode = "none"
	}
	if s.PanicAction != "minimize" {
		s.PanicAction = "hide"
	}
	if s.IdleLockMin < 0 {
		s.IdleLockMin = 0
	}
	pinHash, pinSalt, pinIter := p.PinHash, p.PinSalt, p.PinIter
	p.PrivacySettings = s
	p.PinHash, p.PinSalt, p.PinIter = pinHash, pinSalt, pinIter

	hkErr := a.applyHotkey(p)
	if hkErr != nil {
		p.PanicEnabled = false
	}
	if err := a.savePrivacy(p); err != nil {
		return err
	}
	return hkErr
}

// applyHotkey (пере)регистрирует кнопку паники по настройкам.
func (a *App) applyHotkey(p privacyStored) error {
	if !p.PanicEnabled || p.PanicVK == 0 {
		a.priv.hotkeys.Unregister()
		return nil
	}
	return a.priv.hotkeys.Register(p.PanicMods, p.PanicVK, a.onPanic)
}

// onPanic — нажата кнопка паники: спрятать окно (и заблокировать), повторно — вернуть.
func (a *App) onPanic() {
	a.priv.mu.Lock()
	wasHidden := a.priv.panicHidden
	a.priv.panicHidden = !wasHidden
	a.priv.mu.Unlock()

	if wasHidden {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
		return
	}
	p := a.loadPrivacy()
	if p.LockOnPanic && p.HasPin {
		a.setLocked(true)
	}
	a.emit("privacy-panic")
	if p.PanicAction == "minimize" {
		runtime.WindowMinimise(a.ctx)
	} else {
		runtime.WindowHide(a.ctx)
	}
}

// NotifyWindowShown вызывается фронтендом, когда окно снова получило фокус
// (например, развернули из панели задач после «свернуть по панике»).
func (a *App) NotifyWindowShown() {
	a.priv.mu.Lock()
	a.priv.panicHidden = false
	a.priv.mu.Unlock()
}

// showFromSecondInstance — повторный запуск exe возвращает спрятанное окно.
func (a *App) showFromSecondInstance() {
	a.NotifyWindowShown()
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) setLocked(v bool) {
	a.priv.mu.Lock()
	a.priv.locked = v
	a.priv.mu.Unlock()
	if v {
		a.emit("privacy-locked")
	}
}

// emit шлёт событие во фронтенд; вне запущенного Wails (тесты) — ничего не делает,
// иначе runtime.EventsEmit завершает процесс из-за «чужого» контекста.
func (a *App) emit(name string) {
	if a.live {
		runtime.EventsEmit(a.ctx, name)
	}
}

// Lock блокирует лаунчер (только если задан PIN).
func (a *App) Lock() error {
	if !a.loadPrivacy().HasPin {
		return fmt.Errorf("set a PIN first")
	}
	a.setLocked(true)
	return nil
}

// Unlock снимает блокировку при верном PIN.
func (a *App) Unlock(pin string) error {
	if err := a.checkPin(pin); err != nil {
		return err
	}
	a.setLocked(false)
	return nil
}

// VerifyPin проверяет PIN без смены состояния (показ скрытых коллекций).
// Без заданного PIN всегда успешно.
func (a *App) VerifyPin(pin string) error {
	if !a.loadPrivacy().HasPin {
		return nil
	}
	return a.checkPin(pin)
}

// SetPin задаёт, меняет или (при пустом newPin) снимает PIN. Если PIN уже был,
// нужен текущий.
func (a *App) SetPin(oldPin, newPin string) error {
	p := a.loadPrivacy()
	if p.HasPin {
		if err := a.checkPin(oldPin); err != nil {
			return err
		}
	}
	newPin = strings.TrimSpace(newPin)
	if newPin == "" {
		p.PinHash, p.PinSalt, p.PinIter = "", "", 0
		return a.savePrivacy(p)
	}
	if len([]rune(newPin)) < pinMinLength {
		return fmt.Errorf("PIN must be at least %d characters", pinMinLength)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash, err := hashPin(newPin, salt, pinIterations)
	if err != nil {
		return err
	}
	p.PinHash = base64.StdEncoding.EncodeToString(hash)
	p.PinSalt = base64.StdEncoding.EncodeToString(salt)
	p.PinIter = pinIterations
	return a.savePrivacy(p)
}

func hashPin(pin string, salt []byte, iter int) ([]byte, error) {
	return pbkdf2.Key(sha256.New, pin, salt, iter, 32)
}

// checkPin сверяет PIN с ограничением перебора: после pinMaxAttempts ошибок —
// пауза pinCooldown.
func (a *App) checkPin(pin string) error {
	a.priv.mu.Lock()
	if a.priv.fails >= pinMaxAttempts {
		if wait := pinCooldown - time.Since(a.priv.lastFail); wait > 0 {
			a.priv.mu.Unlock()
			return fmt.Errorf("too many attempts, wait %d s", int(wait.Seconds())+1)
		}
		a.priv.fails = 0
	}
	a.priv.mu.Unlock()

	p := a.loadPrivacy()
	if !p.HasPin {
		return nil
	}
	salt, err1 := base64.StdEncoding.DecodeString(p.PinSalt)
	want, err2 := base64.StdEncoding.DecodeString(p.PinHash)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("stored PIN is damaged")
	}
	got, err := hashPin(strings.TrimSpace(pin), salt, p.PinIter)
	if err != nil {
		return err
	}
	a.priv.mu.Lock()
	defer a.priv.mu.Unlock()
	if subtle.ConstantTimeCompare(got, want) != 1 {
		a.priv.fails++
		a.priv.lastFail = time.Now()
		return fmt.Errorf("wrong PIN")
	}
	a.priv.fails = 0
	return nil
}
