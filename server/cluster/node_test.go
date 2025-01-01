package cluster

import (
	"testing"
	"time"

	"github.com/andydunstall/piko/bench/config"
)

func TestMaybeShedConnections(t *testing.T) {
	node := &Node{
		ID:        "test-node",
		Endpoints: map[string]int{"ep1": 50, "ep2": 30},
	}

	metadata := []*NodeMetadata{
		{ID: "node1", Upstreams: 80},
		{ID: "node2", Upstreams: 40},
	}

	average := float64(60)
	totalConnections := 120
	threshold := 0.1 // 10% above average
	shedRate := 0.05 // Shed 5% per iteration

	node.maybeShedConnections(metadata, average, threshold, shedRate, totalConnections)

	// Verify connection counts after shedding.
	totalAfter := 0
	for _, count := range node.Endpoints {
		totalAfter += count
	}
	if totalAfter >= 80 {
		t.Errorf("node did not shed enough connections, remaining: %d", totalAfter)
	}
}

func TestStartRebalancing(t *testing.T) {
	nodes := []*Node{
		{ID: "node1", Endpoints: map[string]int{"endpoint1": 10}},
		{ID: "node2", Endpoints: map[string]int{"endpoint2": 20}},
		{ID: "node3", Endpoints: map[string]int{"endpoint3": 90}},
	}

	// Mocked configuration.
	cfg := config.Config{
		RebalanceThreshold: 0.1,  // 10% above average
		ShedRate:           0.05, // 5% per second
	}

	// Node instance for testing.
	testNode := nodes[0]

	go testNode.StartRebalancing(nodes, time.Millisecond*500, cfg)

	time.Sleep(time.Second * 2)

	// Verify connection counts were rebalanced.
	if nodes[0].Endpoints["endpoint1"] > 10 {
		t.Errorf("node1 should have shed connections, got %d", nodes[0].Endpoints["ep1"])
	}
}
