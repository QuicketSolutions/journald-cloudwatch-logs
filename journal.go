package main

import (
	"github.com/coreos/go-systemd/v22/sdjournal"
	"strconv"
	"strings"
)

func AddLogFilters(journal *sdjournal.Journal, config *Config) {

	// Add Priority Filters
	if config.LogPriority < DEBUG {
		//lint:ignore S1005 Not sure staticcheck is correct
		for p, _ := range PriorityJSON {
			if p <= config.LogPriority {
				journal.AddMatch("PRIORITY=" + strconv.Itoa(int(p)))
			}
		}
		journal.AddDisjunction()
	}

	// Add unit filter (multiple values possible, separate by ",")
	if config.LogUnit != "" {
		unitsRaw := strings.Split(config.LogUnit, ",")

		for _, unitR := range unitsRaw {
			unit := strings.TrimSpace(unitR)
			if unit != "" {
				if !strings.HasSuffix(unit, ".service") {
					unit += ".service"
				}
				journal.AddMatch("_SYSTEMD_UNIT=" + unit)
				journal.AddDisjunction()
			}
		}

	}
}
