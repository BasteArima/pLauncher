package scanner

import "testing"

func TestSuggestSearchName(t *testing.T) {
	for _, tt := range []struct{ folder, title, version string }{
		{"MoonlitHarbor_v0.3", "Moonlit Harbor", "0.3"},
		{"MoonlitHarbor_v0.3.1a", "Moonlit Harbor", "0.3.1a"},
		{"SilverLake-0.10.2", "Silver Lake", "0.10.2"},
		{"Summer Roads Ep 6", "Summer Roads", "Ep 6"},
		{"Summer Roads Episode 6", "Summer Roads", "Episode 6"},
		{"Summer Roads Ch.3", "Summer Roads", "Ch.3"},
		{"Summer Roads Chapter 3", "Summer Roads", "Chapter 3"},
		{"Summer Roads Final", "Summer Roads", "Final"},
		{"Summer Roads Season 2", "Summer Roads", "Season 2"},
		{"Final Fantasy VII", "Final Fantasy VII", ""},
		{"VRParadise_v1.2_pc", "VR Paradise", "1.2"},
		{"RPGQuest_v2.0_win_x64", "RPG Quest", "2.0"},
		{"Night.City_v0.9_WINDOWS", "Night City", "0.9"},
		{"QuietTown-1.0-linux", "Quiet Town", "1.0"},
		{"QuietTown_v1.0_mac", "Quiet Town", "1.0"},
		{"QuietTown_v1.0_RUS_ENG_Repack", "Quiet Town", "1.0"},
		{"DreamHouse [Developer]", "Dream House", ""},
		{"DreamHouse (Windows)", "Dream House", ""},
		{"DreamHouse {repack}", "Dream House", ""},
		{"DreamHouse [v0.8] [Studio]", "Dream House", "0.8"},
		{"DreamHouse (v0.8) {win}", "Dream House", "0.8"},
		{"DreamHouse_[v0.8]_pc", "Dream House", "0.8"},
		{"DreamHouse_[v0.8]_[Studio]_x64", "Dream House", "0.8"},
		{"DreamHouse [Final]", "Dream House", "Final"},
		{"Blue_Moon-v0.1 [ENG]", "Blue Moon", "0.1"},
		{"СонныйГород_v0.5", "Сонный Город", "0.5"},
		{"CaféAuLait_v1.1", "Café Au Lait", "1.1"},
		{"星の夢_v1.0", "星の夢", "1.0"},
		{"Portal 2", "Portal 2", ""},
		{"Game 2026", "Game 2026", ""},
		{"Roommates", "Roommates", ""},
		{"A Tale (of Two) Worlds_v1.0", "A Tale (of Two) Worlds", "1.0"},
		{"  Blue---Moon___  ", "Blue Moon", ""},
		{"", "", ""},
		{"___", "", ""},
		{"Game_v12", "Game", "12"},
		{"Game_Final_win", "Game", "Final"},
		{"Game_V1.0B", "Game", "1.0B"},
	} {
		t.Run(tt.folder, func(t *testing.T) {
			title, version := SuggestSearchName(tt.folder)
			if title != tt.title || version != tt.version {
				t.Fatalf("got (%q, %q), want (%q, %q)", title, version, tt.title, tt.version)
			}
		})
	}
}
