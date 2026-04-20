package orchestrator

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func ComputeSiteBudget(input BudgetInput) SiteBudget {
	rackLoads := cloneRackLoads(input.CurrentRackLoadKW)
	siteCurrentLoad := sumLoads(rackLoads)
	siteRampPolicy := buildRampPolicy(input.Site)

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
	siteSafeDispatchable := siteAvailable
	if siteRampPolicy.RampUpKWPerInterval > 0 {
		siteSafeDispatchable = minPositive(siteSafeDispatchable, siteRampPolicy.RampUpKWPerInterval)
	}

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
		criticalityClass := normalizeLoadCriticalityClass(string(rack.CriticalityClass))
		criticalityReason := strings.TrimSpace(rack.CriticalityReason)
		if criticalityReason == "" {
			criticalityReason = defaultCriticalityReason(criticalityClass)
		}
		safetyBlocked := rack.SafetyLocked || criticalityClass == LoadClassSafetyBlocked
		safetyReason := strings.TrimSpace(rack.SafetyLockReason)
		if safetyBlocked && safetyReason == "" {
			safetyReason = "rack is blocked until the safety condition is cleared"
		}
		rampUpLimitKW := effectiveRampUpLimit(input.Site, rack)
		rampDownLimitKW := effectiveRampDownLimit(input.Site, rack)
		upRampRemainingKW := rackAssetBudget.AvailableCapacityKW
		downRampRemainingKW := currentLoad
		if rampUpLimitKW > 0 {
			upRampRemainingKW = rampUpLimitKW
		}
		if rampDownLimitKW > 0 {
			downRampRemainingKW = minPositive(currentLoad, rampDownLimitKW)
		}
		if safetyBlocked {
			upRampRemainingKW = 0
			downRampRemainingKW = currentLoad
		}

		pathAvailabilities := []float64{rackAssetBudget.AvailableCapacityKW, siteAvailable}
		flags := make([]string, 0, 8)
		if rackAssetBudget.AvailableCapacityKW <= 0 {
			flags = append(flags, "rack_safe_capacity_exhausted")
		}
		if rackAssetBudget.CurrentLoadKW > rackAssetBudget.SafeCapacityKW {
			flags = append(flags, "rack_current_load_above_safe_capacity")
		}
		if thermalHeadroom <= 0 {
			flags = append(flags, "thermal_density_limit_reached")
		}
		if rack.MaintenanceActive {
			flags = append(flags, "rack_maintenance_active")
		}
		if safetyBlocked {
			flags = append(flags, "rack_safety_blocked")
		}
		if siteRampPolicy.RampUpKWPerInterval > 0 {
			pathAvailabilities = append(pathAvailabilities, siteRampPolicy.RampUpKWPerInterval)
		}
		if rampUpLimitKW > 0 {
			pathAvailabilities = append(pathAvailabilities, rampUpLimitKW)
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
		safeDispatchableKW := minPositive(pathAvailabilities...)
		if safetyBlocked {
			safeDispatchableKW = 0
		}

		rackBudgets = append(rackBudgets, RackBudget{
			RackID:                rack.RackID,
			BusID:                 rack.BusID,
			FeederID:              rack.FeederID,
			PDUID:                 rack.PDUID,
			CriticalityClass:      criticalityClass,
			CriticalityRank:       criticalityRank(criticalityClass),
			CriticalityReason:     criticalityReason,
			SafetyBlocked:         safetyBlocked,
			SafetyBlockReason:     safetyReason,
			CurrentLoadKW:         currentLoad,
			NominalCapacityKW:     rackAssetBudget.NominalCapacityKW,
			EffectiveCapacityKW:   rackAssetBudget.EffectiveCapacityKW,
			ReservedCapacityKW:    rackAssetBudget.ReservedCapacityKW,
			SafeCapacityKW:        rackAssetBudget.SafeCapacityKW,
			AvailableCapacityKW:   rackAssetBudget.AvailableCapacityKW,
			ThermalDensityLimitKW: rack.ThermalDensityLimitKW,
			ThermalHeadroomKW:     thermalHeadroom,
			RampUpLimitKW:         rampUpLimitKW,
			RampDownLimitKW:       rampDownLimitKW,
			UpRampRemainingKW:     upRampRemainingKW,
			DownRampRemainingKW:   downRampRemainingKW,
			SafeDispatchableKW:    safeDispatchableKW,
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
	if siteRampPolicy.IntervalSeconds > 0 && (siteRampPolicy.RampUpKWPerInterval > 0 || siteRampPolicy.RampDownKWPerInterval > 0) {
		constraintFlags = append(constraintFlags, "site_ramp_policy_active")
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
		SafeDispatchableKW:  siteSafeDispatchable,
		RampPolicy:          siteRampPolicy,
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
	rackCurrentLoad := make(map[string]float64, len(budget.Racks))
	rackUpRampRemaining := make(map[string]float64, len(budget.Racks))
	rackDownRampRemaining := make(map[string]float64, len(budget.Racks))
	for _, rack := range budget.Racks {
		rackRemaining[rack.RackID] = rack.AvailableCapacityKW
		thermalRemaining[rack.RackID] = rack.ThermalHeadroomKW
		rackCurrentLoad[rack.RackID] = rack.CurrentLoadKW
		rackUpRampRemaining[rack.RackID] = rack.UpRampRemainingKW
		rackDownRampRemaining[rack.RackID] = rack.DownRampRemainingKW
	}
	siteRemaining := budget.AvailableCapacityKW
	siteUpRampRemaining := math.MaxFloat64
	if budget.RampPolicy.RampUpKWPerInterval > 0 {
		siteUpRampRemaining = budget.RampPolicy.RampUpKWPerInterval
	}
	siteDownRampRemaining := budget.CurrentLoadKW
	if budget.RampPolicy.RampDownKWPerInterval > 0 {
		siteDownRampRemaining = budget.RampPolicy.RampDownKWPerInterval
	}

	type scheduledRequest struct {
		index   int
		request DispatchRequest
	}

	queue := make([]scheduledRequest, 0, len(requests))
	for idx, request := range requests {
		queue = append(queue, scheduledRequest{index: idx, request: request})
	}
	sort.SliceStable(queue, func(i, j int) bool {
		left := queue[i]
		right := queue[j]
		leftDirection := dispatchDirection(left.request.DeltaKW)
		rightDirection := dispatchDirection(right.request.DeltaKW)
		if leftDirection != rightDirection {
			return leftDirection < rightDirection
		}

		leftRack, leftOK := rackByID[left.request.RackID]
		rightRack, rightOK := rackByID[right.request.RackID]
		if leftDirection < 0 {
			leftPriority := 5
			rightPriority := 5
			if leftOK {
				leftPriority = curtailmentPriority(leftRack.CriticalityClass)
			}
			if rightOK {
				rightPriority = curtailmentPriority(rightRack.CriticalityClass)
			}
			if leftPriority != rightPriority {
				return leftPriority < rightPriority
			}
			if math.Abs(left.request.DeltaKW) != math.Abs(right.request.DeltaKW) {
				return math.Abs(left.request.DeltaKW) > math.Abs(right.request.DeltaKW)
			}
			return left.index < right.index
		}

		leftPriority := -1
		rightPriority := -1
		if leftOK {
			leftPriority = criticalityRank(leftRack.CriticalityClass)
		}
		if rightOK {
			rightPriority = criticalityRank(rightRack.CriticalityClass)
		}
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		return left.index < right.index
	})

	decisions := make([]DispatchDecision, len(requests))
	allViolations := make([]ConstraintViolation, 0)

	for _, scheduled := range queue {
		request := scheduled.request
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
			decision.Explanation = "the requested rack is not present in the current energy inventory, so the action cannot be evaluated safely."
			decisions[scheduled.index] = decision
			continue
		}

		if request.DeltaKW == 0 {
			decision.Explanation = "no power delta was requested, so no operational change is required."
			decisions[scheduled.index] = decision
			continue
		}

		if request.DeltaKW < 0 {
			requestedReductionKW := math.Abs(request.DeltaKW)
			currentLoadKW := rackCurrentLoad[rack.RackID]
			limits := []float64{requestedReductionKW, currentLoadKW}
			violations := make([]ConstraintViolation, 0, 4)

			if currentLoadKW < requestedReductionKW {
				violations = append(violations, makeViolation("rack", rack.RackID, "rack_no_load_to_reduce", "requested curtailment is larger than the rack current load", currentLoadKW, currentLoadKW, request.DeltaKW, currentLoadKW))
			}
			if !rack.SafetyBlocked && budget.RampPolicy.RampDownKWPerInterval > 0 {
				limits = append(limits, siteDownRampRemaining)
				if siteDownRampRemaining < requestedReductionKW {
					violations = append(violations, makeViolation("site", budget.SiteID, "site_ramp_down_limit", "requested curtailment would exceed the site down-ramp policy for the interval", budget.RampPolicy.RampDownKWPerInterval, budget.CurrentLoadKW-siteRemaining, request.DeltaKW, siteDownRampRemaining))
				}
			}
			if !rack.SafetyBlocked && rack.RampDownLimitKW > 0 {
				limits = append(limits, rackDownRampRemaining[rack.RackID])
				if rackDownRampRemaining[rack.RackID] < requestedReductionKW {
					violations = append(violations, makeViolation("rack", rack.RackID, "rack_ramp_down_limit", "requested curtailment would exceed the rack down-ramp policy for the interval", rack.RampDownLimitKW, currentLoadKW, request.DeltaKW, rackDownRampRemaining[rack.RackID]))
				}
			}

			acceptedReductionKW := minPositive(limits...)
			if rack.SafetyBlocked {
				acceptedReductionKW = minPositive(requestedReductionKW, currentLoadKW)
			}
			if acceptedReductionKW < 0 {
				acceptedReductionKW = 0
			}
			decision.AcceptedDeltaKW = -acceptedReductionKW

			switch {
			case acceptedReductionKW <= 0:
				decision.Status = "rejected"
			case acceptedReductionKW < requestedReductionKW:
				decision.Status = "partial"
			default:
				decision.Status = "accepted"
			}

			if decision.Status != "accepted" {
				decision.Violations = dedupeViolations(violations)
				allViolations = append(allViolations, decision.Violations...)
			}

			rackCurrentLoad[rack.RackID] = maxFloat(rackCurrentLoad[rack.RackID]-acceptedReductionKW, 0)
			rackRemaining[rack.RackID] += acceptedReductionKW
			thermalRemaining[rack.RackID] += acceptedReductionKW
			siteRemaining += acceptedReductionKW
			if budget.RampPolicy.RampDownKWPerInterval > 0 && !rack.SafetyBlocked {
				siteDownRampRemaining = maxFloat(siteDownRampRemaining-acceptedReductionKW, 0)
			}
			if rack.RampDownLimitKW > 0 && !rack.SafetyBlocked {
				rackDownRampRemaining[rack.RackID] = maxFloat(rackDownRampRemaining[rack.RackID]-acceptedReductionKW, 0)
			}
			if rack.BusID != "" {
				busRemaining[rack.BusID] += acceptedReductionKW
			}
			if rack.FeederID != "" {
				feederRemaining[rack.FeederID] += acceptedReductionKW
			}
			if rack.PDUID != "" {
				pduRemaining[rack.PDUID] += acceptedReductionKW
			}

			decision.Explanation = buildDispatchExplanation(rack, decision, "curtailment")
			decisions[scheduled.index] = decision
			continue
		}

		if rack.SafetyBlocked {
			violation := makeViolation("rack", rack.RackID, "rack_safety_blocked", readableSafetyReason(rack), 0, rackCurrentLoad[rack.RackID], request.DeltaKW, 0)
			decision.Status = "rejected"
			decision.Violations = []ConstraintViolation{violation}
			decision.Explanation = "the rack is safety-blocked, so upward dispatch is not allowed until the safety condition is cleared."
			allViolations = append(allViolations, violation)
			decisions[scheduled.index] = decision
			continue
		}

		limits := []float64{request.DeltaKW, siteRemaining, rackRemaining[rack.RackID], thermalRemaining[rack.RackID]}
		violations := make([]ConstraintViolation, 0, 6)

		if siteRemaining < request.DeltaKW {
			violations = append(violations, makeViolation("site", budget.SiteID, "site_capacity_exceeded", "requested dispatch would exceed site safe capacity", budget.SafeCapacityKW, budget.CurrentLoadKW, request.DeltaKW, siteRemaining))
		}
		if budget.RampPolicy.RampUpKWPerInterval > 0 {
			limits = append(limits, siteUpRampRemaining)
			if siteUpRampRemaining < request.DeltaKW {
				violations = append(violations, makeViolation("site", budget.SiteID, "site_ramp_up_limit", "requested dispatch would exceed the site up-ramp policy for the interval", budget.RampPolicy.RampUpKWPerInterval, budget.CurrentLoadKW-siteRemaining, request.DeltaKW, siteUpRampRemaining))
			}
		}
		if rack.RampUpLimitKW > 0 {
			limits = append(limits, rackUpRampRemaining[rack.RackID])
			if rackUpRampRemaining[rack.RackID] < request.DeltaKW {
				violations = append(violations, makeViolation("rack", rack.RackID, "rack_ramp_up_limit", "requested dispatch would exceed the rack up-ramp policy for the interval", rack.RampUpLimitKW, rackCurrentLoad[rack.RackID], request.DeltaKW, rackUpRampRemaining[rack.RackID]))
			}
		}

		if rackRemaining[rack.RackID] < request.DeltaKW {
			violations = append(violations, makeViolation("rack", rack.RackID, "rack_safe_capacity_exhausted", "rack safe capacity is exhausted after reserve", rack.SafeCapacityKW, rackCurrentLoad[rack.RackID], request.DeltaKW, rackRemaining[rack.RackID]))
		}
		if thermalRemaining[rack.RackID] < request.DeltaKW {
			violations = append(violations, makeViolation("rack", rack.RackID, "thermal_density_exceeded", "requested dispatch would exceed thermal density limit", rack.ThermalDensityLimitKW, rackCurrentLoad[rack.RackID], request.DeltaKW, thermalRemaining[rack.RackID]))
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
		if budget.RampPolicy.RampUpKWPerInterval > 0 {
			siteUpRampRemaining = maxFloat(siteUpRampRemaining-accepted, 0)
		}
		rackCurrentLoad[rack.RackID] += accepted
		rackRemaining[rack.RackID] = maxFloat(rackRemaining[rack.RackID]-accepted, 0)
		thermalRemaining[rack.RackID] = maxFloat(thermalRemaining[rack.RackID]-accepted, 0)
		if rack.RampUpLimitKW > 0 {
			rackUpRampRemaining[rack.RackID] = maxFloat(rackUpRampRemaining[rack.RackID]-accepted, 0)
		}
		if rack.BusID != "" {
			busRemaining[rack.BusID] = maxFloat(busRemaining[rack.BusID]-accepted, 0)
		}
		if rack.FeederID != "" {
			feederRemaining[rack.FeederID] = maxFloat(feederRemaining[rack.FeederID]-accepted, 0)
		}
		if rack.PDUID != "" {
			pduRemaining[rack.PDUID] = maxFloat(pduRemaining[rack.PDUID]-accepted, 0)
		}

		decision.Explanation = buildDispatchExplanation(rack, decision, "dispatch")
		decisions[scheduled.index] = decision
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

func buildRampPolicy(site SiteProfile) RampPolicy {
	intervalSeconds := site.RampIntervalSeconds
	if intervalSeconds <= 0 && (site.RampUpKWPerInterval > 0 || site.RampDownKWPerInterval > 0) {
		intervalSeconds = 300
	}
	return RampPolicy{
		IntervalSeconds:       intervalSeconds,
		RampUpKWPerInterval:   maxFloat(site.RampUpKWPerInterval, 0),
		RampDownKWPerInterval: maxFloat(site.RampDownKWPerInterval, 0),
	}
}

func effectiveRampUpLimit(site SiteProfile, rack RackProfile) float64 {
	if rack.RampUpKWPerInterval > 0 {
		return rack.RampUpKWPerInterval
	}
	return maxFloat(site.RampUpKWPerInterval, 0)
}

func effectiveRampDownLimit(site SiteProfile, rack RackProfile) float64 {
	if rack.RampDownKWPerInterval > 0 {
		return rack.RampDownKWPerInterval
	}
	return maxFloat(site.RampDownKWPerInterval, 0)
}

func defaultCriticalityReason(class LoadCriticalityClass) string {
	switch normalizeLoadCriticalityClass(string(class)) {
	case LoadClassPreferredProduction:
		return "rack is assigned to preferred production"
	case LoadClassSacrificableLoad:
		return "rack is designated as sacrificial load for curtailment first"
	case LoadClassSafetyBlocked:
		return "rack is blocked by safety policy"
	default:
		return "rack participates in normal production"
	}
}

func dispatchDirection(deltaKW float64) int {
	switch {
	case deltaKW < 0:
		return -1
	case deltaKW > 0:
		return 1
	default:
		return 0
	}
}

func buildDispatchExplanation(rack RackBudget, decision DispatchDecision, action string) string {
	switch decision.Status {
	case "accepted":
		return fmtDispatchAccepted(rack, decision, action)
	case "partial":
		if len(decision.Violations) == 0 {
			return fmtDispatchAccepted(rack, decision, action)
		}
		return "the request was partially accepted because one or more electrical, thermal or ramp limits became active before the full delta could be applied."
	default:
		if len(decision.Violations) == 0 {
			return "the request was rejected because it is not safe to execute in the current operating posture."
		}
		return decision.Violations[0].Message
	}
}

func fmtDispatchAccepted(rack RackBudget, decision DispatchDecision, action string) string {
	if action == "curtailment" {
		return fmt.Sprintf("curtailment of %.2f kW on rack %s fits the current ramp and safety posture.", math.Abs(decision.AcceptedDeltaKW), rack.RackID)
	}
	return fmt.Sprintf("dispatch increase of %.2f kW on rack %s fits the current electrical headroom, thermal envelope and ramp policy.", decision.AcceptedDeltaKW, rack.RackID)
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
