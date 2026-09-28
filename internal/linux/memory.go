package linux

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/marcospjr07/docktor/internal/check"
)

type memoryCheck struct {
	readFile readFileFunc
}

type memoryStats struct {
	totalKB     uint64
	availableKB uint64
}

func (memoryCheck) Name() string { return "Memory" }

func (c memoryCheck) Run(ctx context.Context) check.Result {
	if result, interrupted := contextWarning(ctx); interrupted {
		return result
	}
	data, err := c.readFile("/proc/meminfo")
	if err != nil {
		return check.Result{Status: check.StatusWarn, Message: "unavailable: " + err.Error()}
	}
	stats, err := parseMeminfo(data)
	if err != nil {
		return check.Result{Status: check.StatusWarn, Message: "unavailable: " + err.Error()}
	}
	usedKB := stats.totalKB - stats.availableKB
	percent := 100 * float64(usedKB) / float64(stats.totalKB)
	return check.Result{
		Status: statusForPercent(percent),
		Message: fmt.Sprintf("%.1f / %.1f GiB used (%.1f%%)",
			float64(usedKB)/(1024*1024), float64(stats.totalKB)/(1024*1024), percent),
	}
}

func parseMeminfo(data []byte) (memoryStats, error) {
	var stats memoryStats
	var haveTotal, haveAvailable bool
	for _, line := range strings.Split(string(data), "\n") {
		key, raw, ok := strings.Cut(line, ":")
		if !ok || (key != "MemTotal" && key != "MemAvailable") {
			continue
		}
		fields := strings.Fields(raw)
		if len(fields) != 2 || fields[1] != "kB" {
			return memoryStats{}, fmt.Errorf("invalid %s entry", key)
		}
		value, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return memoryStats{}, fmt.Errorf("invalid %s value: %w", key, err)
		}
		if key == "MemTotal" {
			stats.totalKB = value
			haveTotal = true
		} else {
			stats.availableKB = value
			haveAvailable = true
		}
	}
	if !haveTotal || !haveAvailable {
		return memoryStats{}, errors.New("missing MemTotal or MemAvailable")
	}
	if stats.totalKB == 0 || stats.availableKB > stats.totalKB {
		return memoryStats{}, errors.New("inconsistent memory totals")
	}
	return stats, nil
}
