package orchestrator

import (
	"math"
	"sort"
	"strings"
)

func ComputeSiteBudget(input BudgetInput) SiteBudget {
	rackLoads := cloneRackLoads(input.CurrentRackLoadKW)
	siteCurrentLoad := sumLoads(rackLoads)

	siteAsset := CapacityAsset{
		ID:                     input.Site.SiteID,
		Kind:                   AssetKindSite,
		SiteID:                 input.Site.SiteID,
		Name:                   input.Site.CampusName,
		NominalCapacityKW:      input.Site.TargetCapacityMW * 1000,
		OperatingMarginPct:     input.Site.OperatingReservePct,
		AmbientDerateStartC:    input.Site.AmbientDerateStartC,
		AmbientDeratePctPerDeg: input.Site.AmbientDeratePctPerDeg,
		Status:                 StatusActive,
	}

	busParentByID := make(map[string]string, len(input.Buses))
	for _, bus := range input.Buses {
		busParentByID[bus.ID] = bus.ParentID
	}
	transformerLoads := make(map[string]float64)
	for _, rack := range input.Racks {
		parentID := strings.TrimSpace(busParentByID[rack.BusID])
		if parentID == "" {
			continue
		}
		transformerLoads[parentID] += rackLoads[rack.RackID]
	}
	busLoads := mapLoadsByAsset(input.Racks, rackLoads, func(r RackProfile) string { return r.BusID })
	feederLoads := mapLoadsByAsset(input.Racks, rackLoads, func(r RackProfile) string { return r.FeederID })
	pduLoads := mapLoadsByAsset(input.Racks, rackLoads, func(r RackProfile) string { return r.PDUID })

	transformerBudgets := buildAssetBudgets(input.Transformers, transformerLoads, input.AmbientCelsius)
	busBudgets := buildAssetBudgets(input.Buses, busLoads, input.AmbientCelsius)
	feederBudgets := buildAssetBudgets(input.Feeders, feederLoads, input.AmbientCelsius)
	pduBudgets := buildAssetBudgets(input.PDUs, pduLoads, input.AmbientCelsius)

	siteBudget := computeAssetBudget(siteAsset, siteCurrentLoad, input.AmbientCelsius)

	layerNominals := []float64{siteBudget.NominalCapacityKW}
	layerEffective := []float64{siteBudget.EffectiveCapacityKW}
	layerSafe := []float64{siteBudget.SafeCapacityKW}

	appendLayerTotals := func(items []AssetBudget) {
		totalNominal := sumAssetField(items, func(item AssetBudget) float64 { return item.NominalCapacityKW })
		totalEffective := sumAssetField(items, func(item AssetBudget) float64 { return item.EffectiveCapacityKW })
		totalSafe := sumAssetField(items, func(item AssetBudget) float64 { return item.SafeCapacityKW })

		if totalNominal > 0 {
			layerNominals = append(layerNominals, totalNominal)
		}
		if totalEffective > 0 {
			layerEffective = append(layerEffective, totalEffective)
		}
		if totalSafe > 0 {
			layerSafe = append(layerSafe, totalSafe)
		}
	}

	appendLayerTotals(transformerBudgets)
	appendLayerTotals(busBudgets)
	appendLayerTotals(feederBudgets)
	appendLayerTotals(pduBudgets)

	siteNominal := minPositive(layerNominals...)
	siteEffective := minPositive(layerEffective...)
	siteSafe := minPositive(layerSafe...)
	siteReserved := maxFloat(siteEffective-siteSafe, 0)
	siteAvailable := maxFloat(siteSafe-siteCurrentLoad, 0)

	busBudgetByID := indexAssetBudgets(busBudgets)
	feederBudgetByID := indexAssetBudgets(feederBudgets)
	pduBudgetByID := indexAssetBudgets(pduBudgets)

	rackBudgets := make([]RackBudget, 0, len(input.Racks))
	for _, rack := range input.Racks {
		currentLoad := rackLoads[rack.RackID]
		rackAsset := CapacityAsset{
			ID:                     rack.RackID,
			Kind:                   AssetKindRack,
			SiteID:                 rack.SiteID,
			Name:                   rack.RackID,
			NominalCapacityKW:      rack.NominalCapacityKW,
			OperatingMarginPct:     rack.OperatingMarginPct,
			AmbientDerateStartC:    input.Site.AmbientReferenceC + 1000,
			AmbientDeratePctPerDeg: 0,
			Status:                 rack.Status,
			MaintenanceActive:      rack.MaintenanceActive,
		}
		rackAssetBudget := computeAssetBudget(rackAsset, currentLoad, input.AmbientCelsius)
		thermalHeadroom := maxFloat(rack.ThermalDensityLimitKW-currentLoad, 0)

		pathAvailabilities := []float64{rackAssetBudget.AvailableCapacityKW, siteAvailable}
		flags := make([]string, 0, 4)
		if rackAssetBudget.AvailableCapacityKW <= 0 {
			flags = append(flags, "rack_safe_capacity_exhausted")
		}
		if thermalHeadroom <= 0 {
			flags = append(flags, "thermal_density_limit_reached")
		}

		if rack.BusID != "" {
			if asset, ok := busBudgetByID[rack.BusID]; ok {
				pathAvailabilities = append(pathAvailabilities, asset.AvailableCapacityKW)
				if asset.AvailableCapacityKW <= 0 {
					flags = append(flags, "bus_safe_capacity_exhausted")
				}
			}
		}
		if rack.FeederID != "" {
			if asset, ok := feederBudgetByID[rack.FeederID]; ok {
				pathAvailabilities = append(pathAvailabilities, asset.AvailableCapacityKW)
				if asset.AvailableCapacityKW <= 0 {
					flags = append(flags, "feeder_safe_capacity_exhausted")
				}
			}
		}
		if rack.PDUID != "" {
			if asset, ok := pduBudgetByID[rack.PDUID]; ok {
				pathAvailabilities = append(pathAvailabilities, asset.AvailableCapacityKW)
				if asset.AvailableCapacityKW <= 0 {
					flags = append(flags, "pdu_safe_capacity_exhausted")
				}
			}
		}
		pathAvailabilities = append(pathAvailabilities, thermalHeadroom)

		rackBudgets = append(rackBudgets, RackBudget{
			RackID:                rack.RackID,
			BusID:                 rack.BusID,
			FeederID:              rack.FeederID,
			PDUID:                 rack.PDUID,
			CurrentLoadKW:         currentLoad,
			NominalCapacityKW:     rackAssetBudget.NominalCapacityKW,
			EffectiveCapacityKW:   rackAssetBudget.EffectiveCapacityKW,
			ReservedCapacityKW:    rackAssetBudget.ReservedCapacityKW,
			SafeCapacityKW:        rackAssetBudget.SafeCapacityKW,
			AvailableCapacityKW:   rackAssetBudget.AvailableCapacityKW,
			ThermalDensityLimitKW: rack.ThermalDensityLimitKW,
			ThermalHeadroomKW:     thermalHeadroom,
			SafeDispatchableKW:    minPositive(pathAvailabilities...),
			ConstraintFlags:       dedupeAndSort(flags),
		})
	}

	sort.Slice(rackBudgets, func(i, j int) bool {
		return rackBudgets[i].RackID < rackBudgets[j].RackID
	})

	constraintFlags := make([]string, 0, 8)
	if siteAvailable <= 0 {
		constraintFlags = append(constraintFlags, "site_safe_capacity_exhausted")
	}
	if siteCurrentLoad > siteSafe {
		constraintFlags = append(constraintFlags, "site_current_load_above_safe_capacity")
	}

	policyMode := strings.TrimSpace(input.Site.AdvisoryMode)
	if policyMode == "" {
		policyMode = "advisory-first"
	}

	return SiteBudget{
		SiteID:              input.Site.SiteID,
		PolicyMode:          policyMode,
		CalculatedAt:        input.At,
		TelemetryObservedAt: input.TelemetryObservedAt,
		AmbientCelsius:      input.AmbientCelsius,
		NominalCapacityKW:   siteNominal,
		EffectiveCapacityKW: siteEffective,
		ReservedCapacityKW:  siteReserved,
		SafeCapacityKW:      siteSafe,
		CurrentLoadKW:       siteCurrentLoad,
		AvailableCapacityKW: siteAvailable,
		SafeDispatchableKW:  siteAvailable,
		ConstraintFlags:     dedupeAndSort(constraintFlags),
		Substations:         input.Substations,
		Transformers:        transformerBudgets,
		Buses:               busBudgets,
		Feeders:             feederBudgets,
		PDUs:                pduBudgets,
		Racks:               rackBudgets,
	}
}

func ValidateDispatch(budget SiteBudget, requests []DispatchRequest, requestedBy string) DispatchValidationResult {
	rackByID := make(map[string]RackBudget, len(budget.Racks))
	for _, rack := range budget.Racks {
		rackByID[rack.RackID] = rack
	}

	busRemaining := remainingByAsset(budget.Buses)
	feederRemaining := remainingByAsset(budget.Feeders)
	pduRemaining := remainingByAsset(budget.PDUs)
	rackRemaining := make(map[string]float64, len(budget.Racks))
	thermalRemaining := make(map[string]float64, len(budget.Racks))
	for _, rack := range budget.Racks {
		rackRemaining[rack.RackID] = rack.AvailableCapacityKW
		thermalRemaining[rack.RackID] = rack.ThermalHeadroomKW
	}
	siteRemaining := budget.AvailableCapacityKW

	decisions := make([]DispatchDecision, 0, len(requests))
	allViolations := make([]ConstraintViolation, 0)

	for _, request := range requests {
		decision := DispatchDecision{
			RackID:           request.RackID,
			RequestedDeltaKW: request.DeltaKW,
			Status:           "accepted",
		}

		rack, ok := rackByID[request.RackID]
		if !ok {
			violation := ConstraintViolation{
				Scope:            "rack",
				ScopeID:          request.RackID,
				Code:             "rack_not_found",
				Message:          "rack is not configured in energy inventory",
				RequestedDeltaKW: request.DeltaKW,
			}
			decision.Status = "rejected"
			decision.Violations = []ConstraintViolation{violation}
			allViolations = append(allViolations, violation)
			decisions = append(decisions, decision)
			continue
		}

		if request.DeltaKW <= 0 {
			decision.AcceptedDeltaKW = request.DeltaKW
			decisions = append(decisions, decision)
			continue
		}

		limits := make([]float64, 0, 6)
		violations := make([]ConstraintViolation, 0, 6)

		limits = append(limits, siteRemaining)
		if siteRemaining < request.DeltaKW {
			violations = append(violations, makeViolation("site", budget.SiteID, "site_capacity_exceeded", "requested dispatch would exceed site safe capacity", budget.SafeCapacityKW, budget.CurrentLoadKW, request.DeltaKW, siteRemaining))
		}

		limits = append(limits, rackRemaining[rack.RackID], thermalRemaining[rack.RackID])
		if rackRemaining[rack.RackID] < request.DeltaKW {
			violations = append(violations, makeViolation("rack", rack.RackID, "rack_safe_capacity_exhausted", "rack safe capacity is exhausted after reserve", rack.SafeCapacityKW, rack.CurrentLoadKW, request.DeltaKW, rackRemaining[rack.RackID]))
		}
		if thermalRemaining[rack.RackID] < request.DeltaKW {
			violations = append(violations, makeViolation("rack", rack.RackID, "thermal_density_exceeded", "requested dispatch would exceed thermal density limit", rack.ThermalDensityLimitKW, rack.CurrentLoadKW, request.DeltaKW, thermalRemaining[rack.RackID]))
		}

		if rack.BusID != "" {
			available := busRemaining[rack.BusID]
			limits = append(limits, available)
			if available < request.DeltaKW {
				violations = append(violations, makeViolation("bus", rack.BusID, "bus_capacity_exceeded", "requested dispatch would exceed bus safe capacity", findSafeCapacity(budget.Buses, rack.BusID), findCurrentLoad(budget.Buses, rack.BusID), request.DeltaKW, available))
			}
		}
		if rack.FeederID != "" {
			available := feederRemaining[rack.FeederID]
			limits = append(limits, available)
			if available < request.DeltaKW {
				violations = append(violations, makeViolation("feeder", rack.FeederID, "feeder_capacity_exceeded", "requested dispatch would exceed feeder safe capacity", findSafeCapacity(budget.Feeders, rack.FeederID), findCurrentLoad(budget.Feeders, rack.FeederID), request.DeltaKW, available))
			}
		}
		if rack.PDUID != "" {
			available := pduRemaining[rack.PDUID]
			limits = append(limits, available)
			if available < request.DeltaKW {
				violations = append(violations, makeViolation("pdu", rack.PDUID, "pdu_capacity_exceeded", "requested dispatch would exceed pdu safe capacity", findSafeCapacity(budget.PDUs, rack.PDUID), findCurrentLoad(budget.PDUs, rack.PDUID), request.DeltaKW, available))
			}
		}

		accepted := minPositive(limits...)
		if accepted < 0 {
			accepted = 0
		}
		if accepted > request.DeltaKW {
			accepted = request.DeltaKW
		}

		decision.AcceptedDeltaKW = accepted
		switch {
		case accepted <= 0:
			decision.Status = "rejected"
		case accepted < request.DeltaKW:
			decision.Status = "partial"
		default:
			decision.Status = "accepted"
		}

		if decision.Status != "accepted" {
			decision.Violations = dedupeViolations(violations)
			allViolations = append(allViolations, decision.Violations...)
		}

		siteRemaining = maxFloat(siteRemaining-accepted, 0)
		rackRemaining[rack.RackID] = maxFloat(rackRemaining[rack.RackID]-accepted, 0)
		thermalRemaining[rack.RackID] = maxFloat(thermalRemaining[rack.RackID]-accepted, 0)
		if rack.BusID != "" {
			busRemaining[rack.BusID] = maxFloat(busRemaining[rack.BusID]-accepted, 0)
		}
		if rack.FeederID != "" {
			feederRemaining[rack.FeederID] = maxFloat(feederRemaining[rack.FeederID]-accepted, 0)
		}
		if rack.PDUID != "" {
			pduRemaining[rack.PDUID] = maxFloat(pduRemaining[rack.PDUID]-accepted, 0)
		}

		decisions = append(decisions, decision)
	}

	summaryStatus := "accepted"
	acceptedCount := 0
	for _, decision := range decisions {
		if decision.Status == "accepted" {
			acceptedCount++
			continue
		}
		if decision.Status == "partial" {
			summaryStatus = "partial"
		}
		if decision.Status == "rejected" && summaryStatus == "accepted" {
			summaryStatus = "rejected"
		}
	}
	if acceptedCount > 0 && summaryStatus == "rejected" {
		summaryStatus = "partial"
	}

	return DispatchValidationResult{
		SiteID:        budget.SiteID,
		PolicyMode:    budget.PolicyMode,
		RequestedAt:   budget.CalculatedAt,
		RequestedBy:   requestedBy,
		SummaryStatus: summaryStatus,
		Decisions:     decisions,
		Violations:    dedupeViolations(allViolations),
	}
}

func buildAssetBudgets(assets []CapacityAsset, loads map[string]float64, ambient float64) []AssetBudget {
	budgets := make([]AssetBudget, 0, len(assets))
	for _, asset := range assets {
		budgets = append(budgets, computeAssetBudget(asset, loads[asset.ID], ambient))
	}
	sort.Slice(budgets, func(i, j int) bool {
		return budgets[i].ID < budgets[j].ID
	})
	return budgets
}

func computeAssetBudget(asset CapacityAsset, currentLoadKW, ambient float64) AssetBudget {
	factor := statusFactor(asset.Status)
	if asset.MaintenanceActive {
		factor = 0
	}

	ambientFactor := ambientDerateFactor(ambient, asset.AmbientDerateStartC, asset.AmbientDeratePctPerDeg)
	effective := asset.NominalCapacityKW * factor * ambientFactor
	reserved := effective * (asset.OperatingMarginPct / 100)
	safe := maxFloat(effective-reserved, 0)
	available := maxFloat(safe-currentLoadKW, 0)

	return AssetBudget{
		ID:                  asset.ID,
		Name:                asset.Name,
		Kind:                asset.Kind,
		CurrentLoadKW:       currentLoadKW,
		NominalCapacityKW:   asset.NominalCapacityKW,
		EffectiveCapacityKW: effective,
		ReservedCapacityKW:  reserved,
		SafeCapacityKW:      safe,
		AvailableCapacityKW: available,
		Status:              asset.Status,
		MaintenanceActive:   asset.MaintenanceActive,
	}
}

func mapLoadsByAsset(racks []RackProfile, rackLoads map[string]float64, selector func(RackProfile) string) map[string]float64 {
	loads := make(map[string]float64)
	for _, rack := range racks {
		assetID := selector(rack)
		if strings.TrimSpace(assetID) == "" {
			continue
		}
		loads[assetID] += rackLoads[rack.RackID]
	}
	return loads
}

func statusFactor(status AssetStatus) float64 {
	switch status {
	case StatusActive:
		return 1
	case StatusDegraded:
		return 0.85
	case StatusMaintenance, StatusInactive:
		return 0
	default:
		return 0.75
	}
}

func ambientDerateFactor(ambient, start, pctPerDeg float64) float64 {
	if pctPerDeg <= 0 || ambient <= start {
		return 1
	}
	factor := 1 - (((ambient - start) * pctPerDeg) / 100)
	if factor < 0 {
		return 0
	}
	return factor
}

func sumLoads(values map[string]float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

func cloneRackLoads(values map[string]float64) map[string]float64 {
	cloned := make(map[string]float64, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func sumAssetField(items []AssetBudget, selector func(AssetBudget) float64) float64 {
	total := 0.0
	for _, item := range items {
		total += selector(item)
	}
	return total
}

func minPositive(values ...float64) float64 {
	min := math.MaxFloat64
	found := false
	for _, value := range values {
		if value < 0 {
			continue
		}
		if !found || value < min {
			min = value
			found = true
		}
	}
	if !found {
		return 0
	}
	return min
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func indexAssetBudgets(items []AssetBudget) map[string]AssetBudget {
	indexed := make(map[string]AssetBudget, len(items))
	for _, item := range items {
		indexed[item.ID] = item
	}
	return indexed
}

func remainingByAsset(items []AssetBudget) map[string]float64 {
	remaining := make(map[string]float64, len(items))
	for _, item := range items {
		remaining[item.ID] = item.AvailableCapacityKW
	}
	return remaining
}

func findSafeCapacity(items []AssetBudget, id string) float64 {
	for _, item := range items {
		if item.ID == id {
			return item.SafeCapacityKW
		}
	}
	return 0
}

func findCurrentLoad(items []AssetBudget, id string) float64 {
	for _, item := range items {
		if item.ID == id {
			return item.CurrentLoadKW
		}
	}
	return 0
}

func dedupeAndSort(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	if len(result) == 0 {
		return nil
	}
	return result
}

func makeViolation(scope, scopeID, code, message string, limitKW, currentLoadKW, requestedDeltaKW, availableKW float64) ConstraintViolation {
	return ConstraintViolation{
		Scope:            scope,
		ScopeID:          scopeID,
		Code:             code,
		Message:          message,
		LimitKW:          limitKW,
		CurrentLoadKW:    currentLoadKW,
		RequestedDeltaKW: requestedDeltaKW,
		AvailableKW:      availableKW,
	}
}

func dedupeViolations(values []ConstraintViolation) []ConstraintViolation {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]ConstraintViolation, 0, len(values))
	for _, value := range values {
		key := value.Scope + "|" + value.ScopeID + "|" + value.Code
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		left := result[i].Scope + ":" + result[i].ScopeID + ":" + result[i].Code
		right := result[j].Scope + ":" + result[j].ScopeID + ":" + result[j].Code
		return left < right
	})
	return result
}
