//go:build windows

package main

import (
	"fmt"
	goruntime "runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Глобальная горячая клавиша через WinAPI RegisterHotKey. Регистрация привязана
// к потоку, поэтому держим отдельную горутину с закреплённым OS-потоком и своим
// циклом сообщений; снятие — через WM_QUIT этому потоку.

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
)

const (
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
	modNoRepeat = 0x4000
	hotkeyID    = 1
)

type winMsg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       struct{ X, Y int32 }
	LPrivate uint32
}

type hotkeyManager struct {
	regMu    sync.Mutex // сериализует Register (смена папки данных может вызвать его параллельно)
	mu       sync.Mutex
	threadID uintptr
	done     chan struct{}
}

// Register снимает прежнюю комбинацию и регистрирует новую.
// mods — битовая маска MOD_ALT(1)|MOD_CONTROL(2)|MOD_SHIFT(4)|MOD_WIN(8), vk — virtual-key code.
func (h *hotkeyManager) Register(mods, vk uint32, onPress func()) error {
	h.regMu.Lock()
	defer h.regMu.Unlock()
	h.Unregister()
	errc := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		goruntime.LockOSThread()
		defer goruntime.UnlockOSThread()
		tid, _, _ := procGetCurrentThreadId.Call()
		r, _, callErr := procRegisterHotKey.Call(0, hotkeyID, uintptr(mods|modNoRepeat), uintptr(vk))
		if r == 0 {
			errc <- fmt.Errorf("the key combination is already used by another program (%v)", callErr)
			close(done)
			return
		}
		h.mu.Lock()
		h.threadID, h.done = tid, done
		h.mu.Unlock()
		errc <- nil

		var m winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 { // WM_QUIT или ошибка
				break
			}
			if m.Message == wmHotkey {
				go onPress()
			}
		}
		procUnregisterHotKey.Call(0, hotkeyID)
		close(done)
	}()
	return <-errc
}

// Unregister снимает текущую комбинацию (если есть) и дожидается остановки потока.
func (h *hotkeyManager) Unregister() {
	h.mu.Lock()
	tid, done := h.threadID, h.done
	h.threadID, h.done = 0, nil
	h.mu.Unlock()
	if tid == 0 {
		return
	}
	procPostThreadMessageW.Call(tid, wmQuit, 0, 0)
	<-done
}
