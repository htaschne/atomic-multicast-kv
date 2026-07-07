package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const DefaultPartitionCount = 2

type ClusterConfig struct {
	LocalPartition PartitionID
	PartitionCount int
	Peers          map[PartitionID]string
	Mode           ProtocolMode
}

func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		LocalPartition: 0,
		PartitionCount: DefaultPartitionCount,
		Peers: map[PartitionID]string{
			0: "http://localhost:4000",
			1: "http://localhost:4001",
		},
		Mode: ModeOriginal,
	}
}

func LoadClusterConfigFromEnv() (ClusterConfig, error) {
	cfg := DefaultClusterConfig()

	cfg.LocalPartition = PartitionID(envInt("PARTITION_ID", int(cfg.LocalPartition)))
	cfg.PartitionCount = envInt("PARTITION_COUNT", cfg.PartitionCount)

	mode, err := parseProtocolMode(envString("PROTOCOL_MODE", string(cfg.Mode)))
	if err != nil {
		return ClusterConfig{}, err
	}
	cfg.Mode = mode

	peers, err := parsePeers(envString("PEERS", defaultPeers(cfg.PartitionCount, "localhost")))
	if err != nil {
		return ClusterConfig{}, err
	}
	cfg.Peers = peers

	if err := cfg.Validate(); err != nil {
		return ClusterConfig{}, err
	}
	return cfg, nil
}

func (c ClusterConfig) Validate() error {
	if c.PartitionCount <= 0 {
		return fmt.Errorf("partition count must be positive")
	}
	if int(c.LocalPartition) < 0 || int(c.LocalPartition) >= c.PartitionCount {
		return fmt.Errorf("local partition %d outside partition count %d", c.LocalPartition, c.PartitionCount)
	}
	if len(c.Peers) < c.PartitionCount {
		return fmt.Errorf("peer map has %d entries, want at least %d", len(c.Peers), c.PartitionCount)
	}
	for i := 0; i < c.PartitionCount; i++ {
		id := PartitionID(i)
		if c.Peers[id] == "" {
			return fmt.Errorf("missing peer address for partition %d", id)
		}
	}
	if c.Mode != ModeOriginal && c.Mode != ModeStrengthened {
		return fmt.Errorf("unknown protocol mode: %s", c.Mode)
	}
	return nil
}

func defaultPeers(partitionCount int, hostPrefix string) string {
	if partitionCount <= 0 {
		partitionCount = DefaultPartitionCount
	}
	parts := make([]string, 0, partitionCount)
	for i := 0; i < partitionCount; i++ {
		host := hostPrefix
		if hostPrefix == "partition" {
			host = fmt.Sprintf("partition%d", i)
		}
		parts = append(parts, fmt.Sprintf("%d=http://%s:%d", i, host, 4000+i))
	}
	return strings.Join(parts, ",")
}

func parsePeers(raw string) (map[PartitionID]string, error) {
	peers := make(map[PartitionID]string)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("invalid peer entry %q", part)
		}
		idValue, err := strconv.Atoi(strings.TrimSpace(keyValue[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid peer id %q", keyValue[0])
		}
		if idValue < 0 {
			return nil, fmt.Errorf("peer id must be non-negative: %d", idValue)
		}
		peers[PartitionID(idValue)] = strings.TrimSpace(keyValue[1])
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers configured")
	}
	return peers, nil
}

func envString(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
