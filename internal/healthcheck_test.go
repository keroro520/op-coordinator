package internal

import (
	"fmt"
	"testing"
)

var HealthcheckWindow = 20

func contains(array []int, elem int) bool {
	for _, e := range array {
		if elem == e {
			return true
		}
	}
	return false
}

func testHealthcheck(t *testing.T, hcCnt int, errIndexes []int, expectedHcStat int) {
	node := Node{}
	coordinator := Coordinator{
		config:          Config{HealthCheckWindow: HealthcheckWindow},
		healthchecks:    make(map[string]*map[int]error, 1),
		healthcheckStat: make(map[string]int),
		lastHealthcheck: 0,
	}

	for i := 0; i < hcCnt; i++ {
		if contains(errIndexes, i) {
			errors := map[*Node]error{&node: fmt.Errorf("nah")}
			coordinator.updateHealthchecks(&errors)
		} else {
			errors := map[*Node]error{&node: nil}
			coordinator.updateHealthchecks(&errors)
		}
	}

	if expectedHcStat != coordinator.healthcheckStat[node.name] {
		t.Errorf("expected healthcheck stat: %d, actual: %d", HealthcheckWindow-1, coordinator.healthcheckStat[node.name])
	}
}

func TestUpdateHealthcheck(t *testing.T) {
	t.Run(
		"initialize healthcheck",
		func(t *testing.T) {
			testHealthcheck(t, 1, []int{}, HealthcheckWindow-1)
		},
	)
	t.Run(
		"none errors",
		func(t *testing.T) {
			testHealthcheck(t, 2*HealthcheckWindow, []int{}, 0)
		})
	t.Run(
		"one errors",
		func(t *testing.T) {
			testHealthcheck(t, HealthcheckWindow, []int{0}, 1)
		})
	t.Run(
		"two errors at same index",
		func(t *testing.T) {
			testHealthcheck(t, 2*HealthcheckWindow, []int{0, HealthcheckWindow}, 1)
		})
	t.Run(
		"nil to error at same index",
		func(t *testing.T) {
			testHealthcheck(t, 2*HealthcheckWindow, []int{HealthcheckWindow}, 1)
		})
	t.Run(
		"error to nil at same index",
		func(t *testing.T) {
			testHealthcheck(t, 2*HealthcheckWindow, []int{0}, 0)
		})
}
