package main

import "testing"

func TestProfileByModeSpecificModels(t *testing.T) {
	testCases := []struct {
		mode      string
		wantModel string
	}{
		{mode: "s19xp", wantModel: "S19XP"},
		{mode: "s21", wantModel: "S21"},
		{mode: "m50", wantModel: "M50"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			profile, err := profileByMode(tc.mode, 1)
			if err != nil {
				t.Fatalf("profileByMode returned error: %v", err)
			}
			if profile.Model != tc.wantModel {
				t.Fatalf("expected model %s, got %s", tc.wantModel, profile.Model)
			}
		})
	}
}

func TestMixedProfileDistributionInTwentySlots(t *testing.T) {
	countByModel := map[string]int{}
	for i := 1; i <= 20; i++ {
		profile, err := profileByMode("mixed", i)
		if err != nil {
			t.Fatalf("profileByMode returned error at index %d: %v", i, err)
		}
		countByModel[profile.Model]++
	}

	if got, want := countByModel["S21"], 9; got != want {
		t.Fatalf("expected %d S21 slots, got %d", want, got)
	}
	if got, want := countByModel["S19XP"], 7; got != want {
		t.Fatalf("expected %d S19XP slots, got %d", want, got)
	}
	if got, want := countByModel["M50"], 4; got != want {
		t.Fatalf("expected %d M50 slots, got %d", want, got)
	}
}

func TestValidateConfigRejectsInvalidProfileMode(t *testing.T) {
	cfg := config{
		Miners:         10,
		SiteCount:      1,
		Tick:           2,
		Duration:       5,
		Concurrency:    4,
		Timeout:        1,
		ProfileMode:    "invalid",
		Schedule:       "staggered",
		ScheduleJitter: 0,
	}

	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected validateConfig to fail for invalid profile mode")
	}
}
