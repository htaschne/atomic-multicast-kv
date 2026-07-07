package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type InMemoryTransport struct {
	mu    sync.RWMutex
	nodes map[PartitionID]*Skeen
}

func NewInMemoryTransport() *InMemoryTransport {
	return &InMemoryTransport{nodes: make(map[PartitionID]*Skeen)}
}

func (t *InMemoryTransport) Register(node *Skeen) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.nodes[node.id] = node
	node.SetTransport(t)
}

func (t *InMemoryTransport) Send(ctx context.Context, to PartitionID, msg ProtocolMessage) (ProtocolResponse, error) {
	t.mu.RLock()
	node := t.nodes[to]
	t.mu.RUnlock()
	if node == nil {
		return ProtocolResponse{}, fmt.Errorf("unknown partition %d", to)
	}
	return node.ReceiveProtocol(ctx, msg)
}

type HTTPTransport struct {
	peers  map[PartitionID]string
	client *http.Client
}

func NewHTTPTransport(peers map[PartitionID]string) *HTTPTransport {
	return &HTTPTransport{
		peers: peers,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *HTTPTransport) Send(ctx context.Context, to PartitionID, msg ProtocolMessage) (ProtocolResponse, error) {
	baseURL, ok := t.peers[to]
	if !ok {
		return ProtocolResponse{}, fmt.Errorf("unknown peer partition %d", to)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return ProtocolResponse{}, err
	}

	url := strings.TrimRight(baseURL, "/") + "/internal/protocol"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ProtocolResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return ProtocolResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errorBody struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errorBody)
		if errorBody.Error == "" {
			errorBody.Error = resp.Status
		}
		return ProtocolResponse{}, fmt.Errorf("protocol send to partition %d failed: %s", to, errorBody.Error)
	}

	var protocolResp ProtocolResponse
	if err := json.NewDecoder(resp.Body).Decode(&protocolResp); err != nil {
		return ProtocolResponse{}, err
	}
	return protocolResp, nil
}
