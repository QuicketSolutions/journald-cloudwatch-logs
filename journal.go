package main

import (
	"github.com/coreos/go-systemd/v22/sdjournal"
	"strconv"
	"strings"
)

func AddLogFilters(journal *sdjournal.Journal, config *Config) (bool, error) {

	var added = false
	// Add Priority Filters
	if config.LogPriority < DEBUG {
		//lint:ignore S1005 Not sure staticcheck is correct
		for p, _ := range PriorityJSON {
			if p <= config.LogPriority {
				if err := journal.AddMatch("PRIORITY=" + strconv.Itoa(int(p))); err != nil {
					return false, err
				}
			}
		}
		if err := journal.AddDisjunction(); err != nil {
			return false, err
		}
		added = true
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
				if err := journal.AddMatch("_SYSTEMD_UNIT=" + unit); err != nil {
					return false, err
				}
				if err := journal.AddDisjunction(); err != nil {
					return false, err
				}
				added = true
			}
		}

	}
	return added, nil
}
