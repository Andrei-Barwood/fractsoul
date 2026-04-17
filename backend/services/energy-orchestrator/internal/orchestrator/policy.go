package orchestrator

import (
	"fmt"
	"sort"
	"strings"
)

func BuildOperationalView(budget SiteBudget) OperationalView {
	constraints := collectActiveConstraints(budget)
	recommendations := buildPendingRecommendations(budget)
	blocked := buildBlockedActions(budget)
	explanations := buildDecisionExplanations(budget, recommendations, blocked)

	return OperationalView{
		SiteID:                 budget.SiteID,
		PolicyMode:             budget.PolicyMode,
		CalculatedAt:           budget.CalculatedAt,
		Budget:                 budget,
		ActiveConstraints:      constraints,
		PendingRecommendations: recommendations,
		BlockedActions:         blocked,
		Explanations:           explanations,
	}
}

func normalizeLoadCriticalityClass(value string) LoadCriticalityClass {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(LoadClassPreferredProduction):
		return LoadClassPreferredProduction
	case string(LoadClassSacrificableLoad):
		return LoadClassSacrificableLoad
	case string(LoadClassSafetyBlocked):
		return LoadClassSafetyBlocked
	default:
		return LoadClassNormalProduction
	}
}

func criticalityRank(class LoadCriticalityClass) int {
	switch normalizeLoadCriticalityClass(string(class)) {
	case LoadClassPreferredProduction:
		return 3
	case LoadClassNormalProduction:
		return 2
	case LoadClassSacrificableLoad:
		return 1
	default:
		return 0
	}
}

func curtailmentPriority(class LoadCriticalityClass) int {
	switch normalizeLoadCriticalityClass(string(class)) {
	case LoadClassSafetyBlocked:
		return 1
	case LoadClassSacrificableLoad:
		return 2
	case LoadClassNormalProduction:
		return 3
	default:
		return 4
	}
}

func collectActiveConstraints(budget SiteBudget) []ActiveConstraint {
	items := make([]ActiveConstraint, 0, 4+len(budget.Racks))

	if budget.RampPolicy.IntervalSeconds > 0 && (budget.RampPolicy.RampUpKWPerInterval > 0 || budget.RampPolicy.RampDownKWPerInterval > 0) {
		items = append(items, ActiveConstraint{
			Scope:       "site",
			ScopeID:     budget.SiteID,
			Severity:    "info",
			Code:        "site_ramp_policy_active",
			Summary:     "site ramp policy is active",
			Explanation: fmt.Sprintf("dispatch changes are smoothed over %d seconds with up-ramp %.2f kW and down-ramp %.2f kW per interval", budget.RampPolicy.IntervalSeconds, budget.RampPolicy.RampUpKWPerInterval, budget.RampPolicy.RampDownKWPerInterval),
		})
	}
	if budget.CurrentLoadKW > budget.SafeCapacityKW {
		items = append(items, ActiveConstraint{
			Scope:       "site",
			ScopeID:     budget.SiteID,
			Severity:    "critical",
			Code:        "site_current_load_above_safe_capacity",
			Summary:     "site current load is above safe capacity",
			Explanation: "current site load is above the safe capacity after derating and reserve, so curtailment should be prioritized before any new dispatch.",
			LimitKW:     budget.SafeCapacityKW,
			CurrentKW:   budget.CurrentLoadKW,
			AvailableKW: budget.AvailableCapacityKW,
		})
	}
	if budget.AvailableCapacityKW <= 0 {
		items = append(items, ActiveConstraint{
			Scope:       "site",
			ScopeID:     budget.SiteID,
			Severity:    "warning",
			Code:        "site_dispatch_headroom_exhausted",
			Summary:     "site dispatch headroom is exhausted",
			Explanation: "the site cannot safely accept additional load until current consumption decreases or capacity improves.",
			LimitKW:     budget.SafeCapacityKW,
			CurrentKW:   budget.CurrentLoadKW,
		})
	}

	for _, rack := range budget.Racks {
		if rack.SafetyBlocked {
			items = append(items, ActiveConstraint{
				Scope:       "rack",
				ScopeID:     rack.RackID,
				Severity:    "critical",
				Code:        "rack_safety_blocked",
				Summary:     "rack is blocked by safety policy",
				Explanation: readableSafetyReason(rack),
				CurrentKW:   rack.CurrentLoadKW,
			})
		}
		if rack.ThermalHeadroomKW <= 0 {
			items = append(items, ActiveConstraint{
				Scope:       "rack",
				ScopeID:     rack.RackID,
				Severity:    "warning",
				Code:        "rack_thermal_density_reached",
				Summary:     "rack thermal density limit reached",
				Explanation: "the rack is at or above its thermal density threshold, so any additional load would worsen thermal risk.",
				LimitKW:     rack.ThermalDensityLimitKW,
				CurrentKW:   rack.CurrentLoadKW,
			})
		}
		if rack.SafeDispatchableKW <= 0 && !rack.SafetyBlocked {
			items = append(items, ActiveConstraint{
				Scope:       "rack",
				ScopeID:     rack.RackID,
				Severity:    "warning",
				Code:        "rack_dispatch_headroom_exhausted",
				Summary:     "rack dispatch headroom is exhausted",
				Explanation: "the rack has no safe dispatchable headroom once rack, thermal and upstream electrical limits are considered.",
				LimitKW:     rack.SafeCapacityKW,
				CurrentKW:   rack.CurrentLoadKW,
				AvailableKW: rack.SafeDispatchableKW,
			})
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if severityRank(items[i].Severity) == severityRank(items[j].Severity) {
			if items[i].Scope == items[j].Scope {
				return items[i].ScopeID < items[j].ScopeID
			}
			return items[i].Scope < items[j].Scope
		}
		return severityRank(items[i].Severity) < severityRank(items[j].Severity)
	})

	if len(items) == 0 {
		return nil
	}
	return items
}

func buildPendingRecommendations(budget SiteBudget) []PendingRecommendation {
	remainingReductionKW := maxFloat(budget.CurrentLoadKW-budget.SafeCapacityKW, 0)
	recommendations := make([]PendingRecommendation, 0)

	candidates := make([]RackBudget, 0, len(budget.Racks))
	for _, rack := range budget.Racks {
		candidates = append(candidates, rack)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if curtailmentPriority(candidates[i].CriticalityClass) == curtailmentPriority(candidates[j].CriticalityClass) {
			if candidates[i].CurrentLoadKW == candidates[j].CurrentLoadKW {
				return candidates[i].RackID < candidates[j].RackID
			}
			return candidates[i].CurrentLoadKW > candidates[j].CurrentLoadKW
		}
		return curtailmentPriority(candidates[i].CriticalityClass) < curtailmentPriority(candidates[j].CriticalityClass)
	})

	for _, rack := range candidates {
		if rack.SafetyBlocked && rack.CurrentLoadKW > 0 {
			reduction := rack.CurrentLoadKW
			if rack.DownRampRemainingKW > 0 {
				reduction = minPositive(reduction, rack.DownRampRemainingKW)
			}
			recommendations = append(recommendations, PendingRecommendation{
				RackID:             rack.RackID,
				Action:             "isolate",
				CriticalityClass:   rack.CriticalityClass,
				PriorityRank:       1,
				RequestedDeltaKW:   -rack.CurrentLoadKW,
				RecommendedDeltaKW: -reduction,
				Reason:             "safety block active on rack",
				Explanation:        fmt.Sprintf("rack %s is safety-blocked, so the safest advisory action is to reduce %.2f kW as quickly as allowed.", rack.RackID, reduction),
				ConstraintCodes:    []string{"rack_safety_blocked"},
			})
			remainingReductionKW = maxFloat(remainingReductionKW-reduction, 0)
			continue
		}

		if remainingReductionKW <= 0 {
			continue
		}
		if rack.CurrentLoadKW <= 0 {
			continue
		}

		reductionCap := rack.CurrentLoadKW
		if rack.DownRampRemainingKW > 0 {
			reductionCap = minPositive(reductionCap, rack.DownRampRemainingKW)
		}
		if reductionCap <= 0 {
			continue
		}

		reduction := minPositive(remainingReductionKW, reductionCap)
		if reduction <= 0 {
			continue
		}

		recommendations = append(recommendations, PendingRecommendation{
			RackID:             rack.RackID,
			Action:             "curtail",
			CriticalityClass:   rack.CriticalityClass,
			PriorityRank:       curtailmentPriority(rack.CriticalityClass),
			RequestedDeltaKW:   -remainingReductionKW,
			RecommendedDeltaKW: -reduction,
			Reason:             "site load should be reduced to recover safe headroom",
			Explanation:        fmt.Sprintf("rack %s is classified as %s, so it is part of the curtailment sequence to recover %.2f kW of site headroom.", rack.RackID, rack.CriticalityClass, reduction),
			ConstraintCodes:    rack.ConstraintFlags,
		})
		remainingReductionKW = maxFloat(remainingReductionKW-reduction, 0)
	}

	if remainingReductionKW > 0 {
		recommendations = append(recommendations, PendingRecommendation{
			RackID:             "",
			Action:             "escalate",
			CriticalityClass:   LoadClassPreferredProduction,
			PriorityRank:       5,
			RequestedDeltaKW:   -remainingReductionKW,
			RecommendedDeltaKW: -remainingReductionKW,
			Reason:             "curtailment pool is insufficient to recover all safe headroom",
			Explanation:        "the current site overload cannot be fully absorbed by sacrificial and normal loads, so the remaining gap requires operator review before touching preferred production.",
			ConstraintCodes:    []string{"site_current_load_above_safe_capacity"},
		})
	}

	if len(recommendations) == 0 {
		return nil
	}
	return recommendations
}

func buildBlockedActions(budget SiteBudget) []BlockedAction {
	items := make([]BlockedAction, 0, len(budget.Racks)+1)

	if budget.AvailableCapacityKW <= 0 {
		items = append(items, BlockedAction{
			RackID:           "",
			AttemptedAction:  "dispatch_increase",
			CriticalityClass: LoadClassPreferredProduction,
			Code:             "site_dispatch_headroom_exhausted",
			Reason:           "site available capacity is exhausted",
			Explanation:      "new load increases are currently blocked at site scope because there is no safe available capacity left after reserve and derating.",
		})
	}

	for _, rack := range budget.Racks {
		switch {
		case rack.SafetyBlocked:
			items = append(items, BlockedAction{
				RackID:           rack.RackID,
				AttemptedAction:  "dispatch_increase",
				CriticalityClass: rack.CriticalityClass,
				Code:             "rack_safety_blocked",
				Reason:           readableSafetyReason(rack),
				Explanation:      "the rack is blocked by safety policy, so upward dispatch is not allowed until the safety condition is cleared.",
			})
		case rack.UpRampRemainingKW <= 0:
			items = append(items, BlockedAction{
				RackID:           rack.RackID,
				AttemptedAction:  "dispatch_increase",
				CriticalityClass: rack.CriticalityClass,
				Code:             "rack_ramp_up_limit",
				Reason:           "rack up-ramp limit reached for the current interval",
				Explanation:      fmt.Sprintf("rack %s cannot increase load in this interval because the configured ramp-up policy has already been exhausted.", rack.RackID),
			})
		case rack.SafeDispatchableKW <= 0:
			items = append(items, BlockedAction{
				RackID:           rack.RackID,
				AttemptedAction:  "dispatch_increase",
				CriticalityClass: rack.CriticalityClass,
				Code:             "rack_dispatch_headroom_exhausted",
				Reason:           "rack has no safe dispatchable headroom",
				Explanation:      fmt.Sprintf("rack %s is constrained by rack, thermal or upstream electrical limits, so additional load should not be dispatched.", rack.RackID),
			})
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RackID == items[j].RackID {
			return items[i].Code < items[j].Code
		}
		return items[i].RackID < items[j].RackID
	})

	if len(items) == 0 {
		return nil
	}
	return items
}

func buildDecisionExplanations(budget SiteBudget, recommendations []PendingRecommendation, blocked []BlockedAction) []DecisionExplanation {
	items := make([]DecisionExplanation, 0, 1+len(budget.Racks))
	items = append(items, DecisionExplanation{
		Scope:       "site",
		ScopeID:     budget.SiteID,
		Severity:    siteExplanationSeverity(budget),
		Title:       "Site operating posture",
		Explanation: fmt.Sprintf("site %s is running at %.2f kW over a safe ceiling of %.2f kW, with %.2f kW still safely dispatchable and ramp policy %.2f/%.2f kW per %ds.", budget.SiteID, budget.CurrentLoadKW, budget.SafeCapacityKW, budget.SafeDispatchableKW, budget.RampPolicy.RampUpKWPerInterval, budget.RampPolicy.RampDownKWPerInterval, budget.RampPolicy.IntervalSeconds),
	})

	for _, rack := range budget.Racks {
		items = append(items, DecisionExplanation{
			Scope:       "rack",
			ScopeID:     rack.RackID,
			Severity:    rackExplanationSeverity(rack),
			Title:       "Rack dispatch explanation",
			Explanation: rackExplanation(rack),
		})
	}

	for _, recommendation := range recommendations {
		items = append(items, DecisionExplanation{
			Scope:       "recommendation",
			ScopeID:     recommendation.RackID,
			Severity:    "warning",
			Title:       "Pending recommendation",
			Explanation: recommendation.Explanation,
		})
	}
	for _, blockedAction := range blocked {
		items = append(items, DecisionExplanation{
			Scope:       "blocked_action",
			ScopeID:     blockedAction.RackID,
			Severity:    "critical",
			Title:       "Blocked action",
			Explanation: blockedAction.Explanation,
		})
	}

	return items
}

func readableSafetyReason(rack RackBudget) string {
	reason := strings.TrimSpace(rack.SafetyBlockReason)
	if reason == "" {
		return "rack is blocked because a safety interlock is active."
	}
	return reason
}

func rackExplanation(rack RackBudget) string {
	parts := []string{
		fmt.Sprintf("rack %s is classified as %s", rack.RackID, rack.CriticalityClass),
	}
	if strings.TrimSpace(rack.CriticalityReason) != "" {
		parts = append(parts, rack.CriticalityReason)
	}
	if rack.SafetyBlocked {
		parts = append(parts, readableSafetyReason(rack))
	}
	if rack.SafeDispatchableKW <= 0 {
		parts = append(parts, "it has no safe upward dispatch headroom at the moment")
	} else {
		parts = append(parts, fmt.Sprintf("it can safely accept up to %.2f kW", rack.SafeDispatchableKW))
	}
	if rack.ThermalHeadroomKW <= 0 {
		parts = append(parts, "thermal density is already exhausted")
	} else {
		parts = append(parts, fmt.Sprintf("thermal headroom is %.2f kW", rack.ThermalHeadroomKW))
	}
	if rack.UpRampRemainingKW >= 0 || rack.DownRampRemainingKW >= 0 {
		parts = append(parts, fmt.Sprintf("the current interval allows %.2f kW up and %.2f kW down ramps", rack.UpRampRemainingKW, rack.DownRampRemainingKW))
	}
	return strings.Join(parts, "; ") + "."
}

func severityRank(value string) int {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "critical":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func siteExplanationSeverity(budget SiteBudget) string {
	switch {
	case budget.CurrentLoadKW > budget.SafeCapacityKW:
		return "critical"
	case budget.AvailableCapacityKW <= 0:
		return "warning"
	default:
		return "info"
	}
}

func rackExplanationSeverity(rack RackBudget) string {
	switch {
	case rack.SafetyBlocked:
		return "critical"
	case rack.SafeDispatchableKW <= 0 || rack.ThermalHeadroomKW <= 0:
		return "warning"
	default:
		return "info"
	}
}
