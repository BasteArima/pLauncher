//go:build windows

package main

import "testing"

// Регистрация/снятие реальной комбинации: Ctrl+Alt+Shift+F24 почти наверняка свободна.
func TestHotkeyRegisterUnregister(t *testing.T) {
	const mods, vkF24 = 1 | 2 | 4, 0x87
	var a, b hotkeyManager
	if err := a.Register(mods, vkF24, func() {}); err != nil {
		t.Skipf("комбинация занята в этой системе: %v", err)
	}
	// Та же комбинация вторым менеджером — конфликт
	if err := b.Register(mods, vkF24, func() {}); err == nil {
		b.Unregister()
		t.Fatal("повторная регистрация занятой комбинации должна падать")
	}
	a.Unregister()
	// После снятия комбинация снова свободна
	if err := b.Register(mods, vkF24, func() {}); err != nil {
		t.Fatalf("после Unregister комбинация не освободилась: %v", err)
	}
	b.Unregister()
	a.Unregister() // повторное снятие безопасно
}
