package main

import "testing"

func TestClusterConfigValidate(t *testing.T) {
	cfg := ClusterConfig{
		LocalPartition: 2,
		PartitionCount: 3,
		Peers: map[PartitionID]string{
			0: "http://p0:4000",
			1: "http://p1:4001",
			2: "http://p2:4002",
		},
		Mode: ModeStrengthened,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestClusterConfigValidateRejectsMissingPeer(t *testing.T) {
	cfg := ClusterConfig{
		LocalPartition: 0,
		PartitionCount: 3,
		Peers: map[PartitionID]string{
			0: "http://p0:4000",
			1: "http://p1:4001",
		},
		Mode: ModeOriginal,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing peer error")
	}
}

func TestParsePeers(t *testing.T) {
	got, err := parsePeers("0=http://p0:4000,1=http://p1:4001,2=http://p2:4002")
	if err != nil {
		t.Fatalf("parsePeers: %v", err)
	}
	if got[2] != "http://p2:4002" {
		t.Fatalf("peer 2=%q, want http://p2:4002", got[2])
	}
}
