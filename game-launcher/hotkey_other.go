//go:build !windows

package main

import (
	"game-launcher/internal/apperr"
)

// На других ОС глобальные горячие клавиши пока не поддерживаются.
type hotkeyManager struct{}

func (h *hotkeyManager) Register(mods, vk uint32, onPress func()) error {
	return apperr.New("hotkey.unsupported", nil, "global hotkeys are supported only on Windows")
}

func (h *hotkeyManager) Unregister() {}
