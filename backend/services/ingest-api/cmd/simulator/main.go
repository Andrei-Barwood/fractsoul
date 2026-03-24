package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	defaultAPIURL       = "http://localhost:8080"
	defaultDuration     = 2 * time.Minute
	defaultTickInterval = 5 * time.Second
	defaultMiners       = 100
	defaultSiteCount    = 2
	defaultConcurrency  = 24
)

type config struct {
	APIURL         string
	APIKey         string
	Duration       time.Duration
	Tick           time.Duration
	Miners         int
	SiteCount      int
	Concurrency    int
	Timeout        time.Duration
	ProfileMode    string
	Schedule       string
	ScheduleJitter time.Duration
}

type failureMode string

const (
	failureNone     failureMode = "none"
	failureOverheat failureMode = "overheat"
	failureHashDrop failureMode = "hash_drop"
)

type minerState struct {
	SiteID    string
	RackID    string
	MinerID   string
	Model     string
	Firmware  string
	BaseHash  float64
	BasePower float64
	BaseTemp  float64
	BaseFan   int
	Phase     float64

	failure      failureMode
	failureTicks int
	rng          *rand.Rand
	failureBias  float64
}

type tickStats struct {
	total    int64
	accepted int64
	failed   int64
}

func main() {
	cfg := loadConfig()
	if err := validateConfig(cfg); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	fleet := buildFleet(cfg.Miners, cfg.SiteCount, cfg.ProfileMode)
	client := &http.Client{Timeout: cfg.Timeout}

	log.Printf(
		"simulator start miners=%d sites=%d duration=%s tick=%s api=%s concurrency=%d",
		cfg.Miners,
		cfg.SiteCount,
		cfg.Duration,
		cfg.Tick,
		cfg.APIURL,
		cfg.Concurrency,
	)

	start := time.Now()
	deadline := start.Add(cfg.Duration)
	tickNumber := 0

	for time.Now().Before(deadline) {
		tickStartedAt := time.Now().UTC()
		stats := runTick(client, cfg, fleet, tickNumber, tickStartedAt)

		log.Printf(
			"tick=%d ts=%s total=%d accepted=%d failed=%d",
			tickNumber,
			tickStartedAt.Format(time.RFC3339),
			atomic.LoadInt64(&stats.total),
			atomic.LoadInt64(&stats.accepted),
			atomic.LoadInt64(&stats.failed),
		)

		tickNumber++
		nextTickAt := tickStartedAt.Add(cfg.Tick)
		if wait := time.Until(nextTickAt); wait > 0 {
			time.Sleep(wait)
		}
	}

	log.Printf("simulator finished ticks=%d elapsed=%s", tickNumber, time.Since(start).Truncate(time.Millisecond))
}

func loadConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.APIURL, "api-url", defaultAPIURL, "Base URL of ingest API")
	flag.StringVar(&cfg.APIKey, "api-key", "", "API key for ingest authentication (optional)")
	flag.DurationVar(&cfg.Duration, "duration", defaultDuration, "Simulation total duration")
	flag.DurationVar(&cfg.Tick, "tick", defaultTickInterval, "Interval between telemetry batches")
	flag.IntVar(&cfg.Miners, "miners", defaultMiners, "Fleet size")
	flag.IntVar(&cfg.SiteCount, "sites", defaultSiteCount, "Number of sites to distribute miners")
	flag.IntVar(&cfg.Concurrency, "concurrency", defaultConcurrency, "Concurrent HTTP requests per tick")
	flag.DurationVar(&cfg.Timeout, "timeout", 5*time.Second, "HTTP timeout per request")
	flag.StringVar(&cfg.ProfileMode, "profile-mode", "mixed", "ASIC profile mode: mixed|s19xp|s21|m50")
	flag.StringVar(&cfg.Schedule, "schedule", "staggered", "Emission scheduler mode: burst|staggered")
	flag.DurationVar(&cfg.ScheduleJitter, "schedule-jitter", 250*time.Millisecond, "Max random jitter applied to scheduled emissions")
	flag.Parse()
	return cfg
}

func validateConfig(cfg config) error {
	if cfg.Miners <= 0 {
		return fmt.Errorf("miners must be > 0")
	}
	if cfg.SiteCount <= 0 {
		return fmt.Errorf("sites must be > 0")
	}
	if cfg.Tick <= 0 {
		return fmt.Errorf("tick must be > 0")
	}
	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be > 0")
	}
	if cfg.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be > 0")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be > 0")
	}
	if _, err := profileByMode(cfg.ProfileMode, 1); err != nil {
		return err
	}
	if cfg.Schedule != "burst" && cfg.Schedule != "staggered" {
		return fmt.Errorf("schedule must be burst or staggered")
	}
	if cfg.ScheduleJitter < 0 {
		return fmt.Errorf("schedule-jitter must be >= 0")
	}
	return nil
}

func runTick(client *http.Client, cfg config, fleet []*minerState, tick int, timestamp time.Time) tickStats {
	var stats tickStats
	var wg sync.WaitGroup
	limiter := make(chan struct{}, cfg.Concurrency)

	for index, miner := range fleet {
		index := index
		miner := miner
		wg.Add(1)
		go func() {
			defer wg.Done()
			if wait := scheduleEmissionWait(cfg, timestamp, len(fleet), index, miner); wait > 0 {
				time.Sleep(wait)
			}
			limiter <- struct{}{}
			defer func() { <-limiter }()

			payload := miner.buildPayload(tick, timestamp)
			atomic.AddInt64(&stats.total, 1)
			if err := postTelemetry(client, cfg.APIURL, cfg.APIKey, payload); err != nil {
				atomic.AddInt64(&stats.failed, 1)
				log.Printf("send failed miner=%s err=%v", miner.MinerID, err)
				return
			}
			atomic.AddInt64(&stats.accepted, 1)
		}()
	}

	wg.Wait()
	return stats
}

func buildFleet(miners, siteCount int, profileMode string) []*minerState {
	fleet := make([]*minerState, 0, miners)
	minersPerSite := int(math.Ceil(float64(miners) / float64(siteCount)))
	racksPerSite := 5

	for index := 1; index <= miners; index++ {
		siteNumber := ((index - 1) / minersPerSite) + 1
		if siteNumber > siteCount {
			siteNumber = siteCount
		}

		positionInSite := ((index - 1) % minersPerSite) + 1
		rackNumber := ((positionInSite - 1) / 10) + 1
		if rackNumber > racksPerSite {
			rackNumber = racksPerSite
		}

		siteID := fmt.Sprintf("site-cl-%02d", siteNumber)
		rackID := fmt.Sprintf("rack-cl-%02d-%02d", siteNumber, rackNumber)
		minerID := fmt.Sprintf("asic-%06d", index)

		seed := time.Now().UnixNano() + int64(index*7919)
		rng := rand.New(rand.NewSource(seed))

		profile, err := profileByMode(profileMode, index)
		if err != nil {
			log.Fatalf("select profile: %v", err)
		}
		baseHash := profile.HashMinTHs + rng.Float64()*(profile.HashMaxTHs-profile.HashMinTHs)
		basePower := profile.PowerMinW + rng.Float64()*(profile.PowerMaxW-profile.PowerMinW)
		baseTemp := profile.TempMinC + rng.Float64()*(profile.TempMaxC-profile.TempMinC)
		baseFan := profile.FanMinRPM + rng.Intn(max(1, profile.FanMaxRPM-profile.FanMinRPM+1))
		firmware := profile.Firmware

		fleet = append(fleet, &minerState{
			SiteID:      siteID,
			RackID:      rackID,
			MinerID:     minerID,
			Model:       profile.Model,
			Firmware:    firmware,
			BaseHash:    baseHash,
			BasePower:   basePower,
			BaseTemp:    baseTemp,
			BaseFan:     baseFan,
			Phase:       rng.Float64() * math.Pi,
			rng:         rng,
			failureBias: profile.FailureBias,
		})
	}

	return fleet
}

func (m *minerState) buildPayload(tick int, ts time.Time) map[string]any {
	load := 0.84 + 0.18*math.Sin((float64(tick)/6.0)+m.Phase) + (m.rng.NormFloat64() * 0.03)
	load = clamp(load, 0.55, 1.20)
	ambient := 24 + 5*math.Sin((float64(tick)/14.0)+(m.Phase*0.5)) + (m.rng.NormFloat64() * 0.8)
	ambient = clamp(ambient, 14, 42)
	freqMHz := clampInt(700+int(load*140)+int(m.rng.NormFloat64()*8), 520, 980)
	voltMV := clampInt(730+int(load*75)+int(m.rng.NormFloat64()*6), 640, 940)

	hashrate := m.BaseHash*load + (m.rng.NormFloat64() * 1.7)
	power := m.BasePower*(0.88+0.24*load) + (m.rng.NormFloat64() * 28)
	temp := m.BaseTemp + (load * 17) + (m.rng.NormFloat64() * 1.9)
	fan := m.BaseFan + int((temp-55)*105) + int(m.rng.NormFloat64()*80)
	fault := string(failureNone)

	if m.failureTicks == 0 {
		r := m.rng.Float64() * m.failureBias
		switch {
		case r < 0.003:
			m.failure = failureOverheat
			m.failureTicks = 4 + m.rng.Intn(7)
		case r < 0.007:
			m.failure = failureHashDrop
			m.failureTicks = 4 + m.rng.Intn(10)
		default:
			m.failure = failureNone
		}
	}

	status := "ok"
	if m.failure != failureNone {
		switch m.failure {
		case failureOverheat:
			temp += 16 + m.rng.Float64()*10
			hashrate *= 0.55
			power *= 1.08
			ambient += 2 + m.rng.Float64()*2
			freqMHz = clampInt(freqMHz-35, 520, 980)
			voltMV = clampInt(voltMV-10, 640, 940)
			status = "critical"
			fault = string(failureOverheat)
		case failureHashDrop:
			hashrate *= 0.35 + m.rng.Float64()*0.2
			power *= 0.78 + m.rng.Float64()*0.1
			temp -= 2 + m.rng.Float64()*3
			freqMHz = clampInt(freqMHz-18, 520, 980)
			voltMV = clampInt(voltMV+8, 640, 940)
			status = "warning"
			fault = string(failureHashDrop)
		}

		m.failureTicks--
		if m.failureTicks <= 0 {
			m.failure = failureNone
		}
	}

	if m.rng.Float64() < 0.0008 {
		// Keep near-zero values instead of exact zero so validator `required`
		// does not reject numeric fields in occasional offline events.
		hashrate = 0.001
		power = 120 + m.rng.Float64()*50
		temp = 39 + m.rng.Float64()*5
		fan = 1
		status = "offline"
		fault = "offline"
	}

	hashrate = clamp(hashrate, 0, 2000)
	power = clamp(power, 0, 10000)
	temp = clamp(temp, -40, 130)
	fan = clampInt(fan, 0, 30000)

	if status == "ok" {
		switch {
		case temp >= 95 || hashrate < m.BaseHash*0.5:
			status = "critical"
		case temp >= 85 || hashrate < m.BaseHash*0.8:
			status = "warning"
		}
	}

	efficiency := 0.0
	if hashrate > 0 {
		efficiency = power / hashrate
	}
	efficiency = clamp(efficiency, 0, 1000)

	return map[string]any{
		"event_id":         uuid.NewString(),
		"timestamp":        ts.Format(time.RFC3339Nano),
		"site_id":          m.SiteID,
		"rack_id":          m.RackID,
		"miner_id":         m.MinerID,
		"firmware_version": m.Firmware,
		"metrics": map[string]any{
			"hashrate_ths":   round(hashrate, 3),
			"power_watts":    round(power, 3),
			"temp_celsius":   round(temp, 3),
			"fan_rpm":        fan,
			"efficiency_jth": round(efficiency, 3),
			"status":         status,
		},
		"tags": map[string]string{
			"source":         "simulator",
			"load_pct":       fmt.Sprintf("%.2f", load*100),
			"fault":          fault,
			"profile":        "asic-v1",
			"sim_version":    "2026.03",
			"asic_model":     m.Model,
			"ambient_temp_c": fmt.Sprintf("%.2f", ambient),
			"freq_mhz":       fmt.Sprintf("%d", freqMHz),
			"volt_mv":        fmt.Sprintf("%d", voltMV),
		},
	}
}

func scheduleEmissionWait(cfg config, tickStartedAt time.Time, fleetSize, minerIndex int, miner *minerState) time.Duration {
	if cfg.Schedule != "staggered" || fleetSize <= 1 || cfg.Tick <= 0 {
		return randomJitter(miner, cfg.ScheduleJitter)
	}

	baseWindow := cfg.Tick
	if baseWindow > 2*time.Second {
		baseWindow = 2 * time.Second
	}
	offsetStep := baseWindow / time.Duration(fleetSize)
	if offsetStep <= 0 {
		offsetStep = time.Millisecond
	}

	plannedAt := tickStartedAt.Add(offsetStep * time.Duration(minerIndex))
	wait := time.Until(plannedAt) + randomJitter(miner, cfg.ScheduleJitter)
	if wait < 0 {
		return 0
	}

	return wait
}

func randomJitter(miner *minerState, maxJitter time.Duration) time.Duration {
	if miner == nil || maxJitter <= 0 {
		return 0
	}
	return time.Duration(miner.rng.Int63n(maxJitter.Nanoseconds() + 1))
}

func postTelemetry(client *http.Client, apiURL, apiKey string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL+"/v1/telemetry/ingest", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post telemetry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d body=%s", resp.StatusCode, string(responseBody))
	}

	return nil
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func round(value float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
