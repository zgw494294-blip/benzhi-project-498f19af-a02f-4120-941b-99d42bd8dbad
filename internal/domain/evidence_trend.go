package domain

import (
	"fmt"
	"sort"
)

const (
	TrendBaseline      = "baseline"
	TrendImproving     = "improving"
	TrendStable        = "stable"
	TrendDeteriorating = "deteriorating"
)

// BuildEvidenceTrend validates and converts every immutable revision for one zone.
func BuildEvidenceTrend(zoneCode string, items []EvidenceRevision) (ZoneEvidenceTrend, error) {
	if len(items) == 0 {
		return ZoneEvidenceTrend{}, ErrNotFound
	}
	revisions := append([]EvidenceRevision(nil), items...)
	sort.SliceStable(revisions, func(i, j int) bool { return revisions[i].Revision < revisions[j].Revision })
	for index, item := range revisions {
		if item.ZoneCode != zoneCode {
			return ZoneEvidenceTrend{}, fmt.Errorf("%w: 污染区 %s 混入修订 %s", ErrDataConsistency, zoneCode, item.ID)
		}
		expected := index + 1
		if item.Revision != expected {
			return ZoneEvidenceTrend{}, fmt.Errorf("%w: 污染区 %s 期望修订号 %d，实际为 %d", ErrDataConsistency, zoneCode, expected, item.Revision)
		}
	}

	result := ZoneEvidenceTrend{ZoneCode: zoneCode, Revisions: make([]EvidenceRevisionTrend, 0, len(revisions))}
	for index, item := range revisions {
		point := EvidenceRevisionTrend{
			EvidenceID: item.ID, Revision: item.Revision, RecordedAt: item.RecordedAt,
			CoveragePercent: item.CoveragePercent, ActivityScore: item.ActivityScore,
			RiskLevel: item.RiskLevel, RiskReasonCodes: append([]string(nil), item.RiskReasonCodes...),
			CoverageDirection: TrendBaseline, ActivityDirection: TrendBaseline,
			RiskDirection: TrendBaseline, Conclusion: TrendBaseline,
		}
		if index > 0 {
			previous := revisions[index-1]
			coverageDelta := item.CoveragePercent - previous.CoveragePercent
			activityDelta := item.ActivityScore - previous.ActivityScore
			point.CoverageDelta, point.ActivityDelta = &coverageDelta, &activityDelta
			point.CoverageDirection = lowerIsBetterDirection(coverageDelta)
			point.ActivityDirection = lowerIsBetterDirection(activityDelta)
			point.RiskDirection = riskDirection(previous.RiskLevel, item.RiskLevel)
			point.Conclusion = combineTrendDirections(point.CoverageDirection, point.ActivityDirection, point.RiskDirection)
		}
		result.Revisions = append(result.Revisions, point)
	}
	latest := revisions[len(revisions)-1]
	result.LatestRiskLevel = latest.RiskLevel
	if len(revisions) == 1 {
		result.OverallDirection = TrendBaseline
	} else {
		first := revisions[0]
		result.OverallDirection = combineTrendDirections(
			lowerIsBetterDirection(latest.CoveragePercent-first.CoveragePercent),
			lowerIsBetterDirection(latest.ActivityScore-first.ActivityScore),
			riskDirection(first.RiskLevel, latest.RiskLevel),
		)
	}
	return result, nil
}

func lowerIsBetterDirection(delta float64) string {
	switch {
	case delta < 0:
		return TrendImproving
	case delta > 0:
		return TrendDeteriorating
	default:
		return TrendStable
	}
}

func riskDirection(previous, current RiskLevel) string {
	delta := riskRank(current) - riskRank(previous)
	switch {
	case delta < 0:
		return TrendImproving
	case delta > 0:
		return TrendDeteriorating
	default:
		return TrendStable
	}
}

func combineTrendDirections(directions ...string) string {
	improving, deteriorating := false, false
	for _, direction := range directions {
		improving = improving || direction == TrendImproving
		deteriorating = deteriorating || direction == TrendDeteriorating
	}
	if improving && !deteriorating {
		return TrendImproving
	}
	if deteriorating && !improving {
		return TrendDeteriorating
	}
	return TrendStable
}
