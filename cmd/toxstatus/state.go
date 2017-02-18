package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

var (
	stateFilename = "nodes.json"
)

func loadState() (*toxStatus, error) {
	bytes, err := ioutil.ReadFile(stateFilename)
	if err != nil {
		switch err.(type) {
		case *os.PathError:
			return new(toxStatus), nil
		default:
			return nil, err
		}
	}

	res := new(toxStatus)
	if err := json.Unmarshal(bytes, res); err != nil {
		return nil, err
	}

	return res, nil
}

func saveState(state *toxStatus) error {
	bytes, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(stateFilename, bytes, 0666)
}

func getState() *toxStatus {
	return &toxStatus{lastScan, lastRefresh, nodes}
}
