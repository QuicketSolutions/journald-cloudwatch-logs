package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreos/go-systemd/v22/sdjournal"
)

func ReadRecords(instanceId string, journal *sdjournal.Journal, c chan<- Record, skip uint64) {
	record := &Record{}

	termC := MakeTerminateChannel()
	checkTerminate := func() bool {
		select {
		case <-termC:
			close(c)
			return true
		default:
			return false
		}
	}

	var lastRecTime time.Time
	for {
		if checkTerminate() {
			return
		}
		err := UnmarshalRecord(journal, record)
		if err != nil {
			c <- synthRecord(
				fmt.Errorf("error unmarshalling record: %s", err),
			)
			continue
		}

		if skip > 0 {
			skip--
		} else {
			record.InstanceId = instanceId
			recTimeNS := int64(record.TimeUsec) * int64(time.Microsecond)
			recTime := time.Unix(recTimeNS / int64(time.Second), recTimeNS % int64(time.Second))
			// Skip records that are more than 14 days old. CWL will not accept them
			if !recTime.Before(time.Now().AddDate(0,0,-14)) {
				// CWL doesn't allow batches with timestamps more than
				// 24 hours apart.
				if recTime.After(lastRecTime.Add(24 * time.Hour)) {
					// Sleep to cause the batch to flush
					time.Sleep(2 * time.Second)
				}
				c <- *record
				lastRecTime = recTime
			}
		}

		for {
			if checkTerminate() {
				return
			}
			seeked, err := journal.Next()
			if err != nil {
				c <- synthRecord(
					fmt.Errorf("error reading from journal: %s", err),
				)
				// It's likely that we didn't actually advance here, so
				// we should wait a bit so we don't spin the CPU at 100%
				// when we run into errors.
				time.Sleep(2 * time.Second)
				continue
			}
			if seeked == 0 {
				// If there's nothing new in the stream then we'll
				// wait for something new to show up.
				// FIXME: We can actually end up waiting up to 2 seconds
				// to gracefully terminate because of this. It'd be nicer
				// to stop waiting if we get a termination signal, but
				// this will do for now.
				journal.Wait(2 * time.Second)
				continue
			}
			break
		}
	}
}

// BatchRecords consumes a channel of individual records and produces
// a channel of slices of record pointers in sizes up to the given
// batch size.
// If records don't show up fast enough, smaller batches will be returned
// each second as long as at least one item is in the buffer.
func BatchRecords(records <-chan Record, batches chan<- []Record, batchSize int) {
	var (
		batch   []Record
		timeout <-chan time.Time
	)
	flush := func() {
		if len(batch) > 0 {
			batches <- batch
		}
		batch = make([]Record, 0, batchSize)
		timeout = time.After(time.Second)
	}
	flush()
	for {
		select {
		case record, ok := <-records:
			if !ok {
				close(batches)
				return
			}
			batch = append(batch, record)
			if len(batch) >= batchSize {
				flush()
			}
		case <-timeout:
			flush()
		}
	}
}

// synthRecord produces synthetic records to report errors, so that
// we can stream our own errors directly into cloudwatch rather than
// emitting them through journald and risking feedback loops.
func synthRecord(err error) Record {
	return Record{
		Command:  "journald-cloudwatch-logs",
		Priority: ERROR,
		Message:  json.RawMessage(err.Error()),
	}
}
