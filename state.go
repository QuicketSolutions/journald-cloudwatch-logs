package main

import (
	"fmt"
	"io"
	"os"
)

const stateFormat = "%s\n%s\n%d\n"

type State struct {
	file *os.File
}

func OpenState(fn string) (State, error) {
	s := State{}
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return s, err
	}
	s.file = f
	return s, nil
}

func (s State) Close() error {
	return s.file.Close()
}

func (s State) Sync() error {
	return s.file.Sync()
}

func (s State) LastState() (string, string, uint64) {
	var bootId string
	var seqToken string
	var lastEventTime uint64

	_, err := s.file.Seek(0, 0)
	if err != nil {
		return "", "", 0
	}
	_, err = fmt.Fscanf(s.file, stateFormat, &bootId, &seqToken, &lastEventTime)
	if err != nil && err != io.EOF  {
		return "", "", 0
	}
	return bootId, seqToken, lastEventTime
}

func (s State) SetState(bootId, seqToken string, lastTime uint64) error {
	_, err := s.file.Seek(0, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.file, stateFormat, bootId, seqToken, lastTime)
	if err != nil {
		return err
	}
	return nil
}
