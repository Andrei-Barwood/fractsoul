package main

import "fmt"

type asicProfile struct {
	Model       string
	Firmware    string
	HashMinTHs  float64
	HashMaxTHs  float64
	PowerMinW   float64
	PowerMaxW   float64
	TempMinC    float64
	TempMaxC    float64
	FanMinRPM   int
	FanMaxRPM   int
	FailureBias float64
}

var defaultProfiles = []asicProfile{
	{
		Model:       "S19XP",
		Firmware:    "stock-2025.11",
		HashMinTHs:  125,
		HashMaxTHs:  150,
		PowerMinW:   2900,
		PowerMaxW:   3250,
		TempMinC:    53,
		TempMaxC:    60,
		FanMinRPM:   5000,
		FanMaxRPM:   6200,
		FailureBias: 1.00,
	},
	{
		Model:       "S21",
		Firmware:    "braiins-2026.1",
		HashMinTHs:  185,
		HashMaxTHs:  220,
		PowerMinW:   3350,
		PowerMaxW:   3850,
		TempMinC:    54,
		TempMaxC:    62,
		FanMinRPM:   5200,
		FanMaxRPM:   6600,
		FailureBias: 0.90,
	},
	{
		Model:       "M50",
		Firmware:    "whatsminer-2026.2",
		HashMinTHs:  110,
		HashMaxTHs:  135,
		PowerMinW:   3050,
		PowerMaxW:   3500,
		TempMinC:    55,
		TempMaxC:    64,
		FanMinRPM:   5300,
		FanMaxRPM:   7000,
		FailureBias: 1.08,
	},
}

func profileByMode(mode string, index int) (asicProfile, error) {
	switch mode {
	case "mixed":
		return mixedProfile(index), nil
	case "s19xp":
		return defaultProfiles[0], nil
	case "s21":
		return defaultProfiles[1], nil
	case "m50":
		return defaultProfiles[2], nil
	default:
		return asicProfile{}, fmt.Errorf("invalid profile-mode %q", mode)
	}
}

func mixedProfile(index int) asicProfile {
	// Weighted rotation for realistic fleet composition.
	// - 45% S21
	// - 35% S19XP
	// - 20% M50
	slot := (index - 1) % 20
	switch {
	case slot < 9:
		return defaultProfiles[1]
	case slot < 16:
		return defaultProfiles[0]
	default:
		return defaultProfiles[2]
	}
}
