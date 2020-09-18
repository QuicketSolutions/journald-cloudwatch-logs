package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/coreos/go-systemd/v22/sdjournal"
)

var help = flag.Bool("help", false, "set to true to show this help")

func main() {
	flag.Parse()

	if versionFlag {
		showVersion()
		os.Exit(0)
	}

	if *help {
		usage()
		os.Exit(0)
	}

	configFilename := flag.Arg(0)
	if configFilename == "" {
		usage()
		os.Exit(1)
	}

	err := run(configFilename)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.Write([]byte{'\n'})
		os.Exit(2)
	}
}

func usage() {
	os.Stderr.WriteString("Usage: journald-cloudwatch-logs <config-file>\n\n")
	flag.PrintDefaults()
	os.Stderr.WriteString("\n")
}

func run(configFilename string) error {
	config, err := LoadConfig(configFilename)
	if err != nil {
		return fmt.Errorf("error reading config: %s", err)
	}

	var journal *sdjournal.Journal
	if config.JournalDir == "" {
		journal, err = sdjournal.NewJournal()
	} else {
		log.Printf("using journal dir: %s", config.JournalDir)
		journal, err = sdjournal.NewJournalFromDir(config.JournalDir)
	}

	if err != nil {
		return fmt.Errorf("error opening journal: %s", err)
	}
	defer journal.Close()

	var filters bool
	filters, err = AddLogFilters(journal, config)

	state, err := OpenState(config.StateFilename)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", config.StateFilename, err)
	}

	lastBootId, nextSeq, lastTime := state.LastState()

	awsSession := config.NewAWSSession()

	writer, err := NewWriter(
		awsSession,
		config.LogGroupName,
		config.LogStreamName,
		nextSeq,
	)
	if err != nil {
		return fmt.Errorf("error initializing writer: %s", err)
	}

	seeked, err := journal.Next()
	if seeked == 0 || err != nil {
		return fmt.Errorf("unable to seek to first item in journal")
	}

	bootId, err := journal.GetData("_BOOT_ID")
	if err != nil {
		return fmt.Errorf("unable to retrieve Boot ID: %w", err)
	}
	bootId = bootId[9:] // Trim off "_BOOT_ID=" prefix

	// If the boot id has changed since our last run then we'll start from
	// the beginning of the stream, but if we're starting up with the same
	// boot id then we'll seek to the end of the stream to avoid repeating
	// anything. However, we will miss any items that were added while we
	// weren't running.
	var lastEntryTime = uint64(0)
	if bootId == lastBootId {
		// If we're still in the same "boot" as we were last time then
		// we were stopped and started again, so we'll seek to the last
		// item in the log as an approximation of resuming streaming,
		// though we will miss any logs that were added while we were
		// running.
		if err = journal.SeekTail(); err != nil {
			return err
		}

		// SeekTail() is a no-op if filters have been added.
		// Consume entries until after the timestamp we've seen.
		if filters {
			var entry *sdjournal.JournalEntry
			lastTimeNS := int64(lastTime) * int64(time.Microsecond)
			lastDate := time.Unix(
				lastTimeNS/int64(time.Second),
				lastTimeNS%int64(time.Second),
			)

			for n, err := journal.Next(); err != nil && n > 0; n, err = journal.Next() {
				if entry, err = journal.GetEntry(); err != nil {
					return err
				}

				lastEntryTime = entry.RealtimeTimestamp
				rtNs := int64(entry.RealtimeTimestamp) * int64(time.Microsecond)
				entryTime := time.Unix(
					rtNs*int64(time.Second),
					rtNs%int64(time.Second),
				)

				if entryTime.After(lastDate) {
					break
				}
			}
		}

		if _, err = journal.Next(); err != nil {
			return err
		}
	}

	err = state.SetState(bootId, nextSeq, lastEntryTime)
	if err != nil {
		return fmt.Errorf("failed to write state: %s", err)
	}

	bufSize := config.BufferSize

	records := make(chan Record)
	batches := make(chan []Record)

	go ReadRecords(config.EC2InstanceId, journal, records, 0)
	go BatchRecords(records, batches, bufSize)

	var thisEntryTime uint64
	for batch := range batches {

		nextSeq, thisEntryTime, err = writer.WriteBatch(batch)
		if err != nil {
			return fmt.Errorf("failed to write to cloudwatch: %s", err)
		}
		if thisEntryTime > 0 {
			lastEntryTime = thisEntryTime
		}

		err = state.SetState(bootId, nextSeq, lastEntryTime)
		if err != nil {
			return fmt.Errorf("failed to write state: %s", err)
		}

	}

	// We fall out here when interrupted by a signal.
	// Last chance to write the state.
	err = state.SetState(bootId, nextSeq, lastEntryTime)
	if err != nil {
		return fmt.Errorf("failed to write state on exit: %s", err)
	}

	return nil
}
