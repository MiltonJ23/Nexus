package service

import (
	"Nexus/internal/adapters/metrics"
	"Nexus/internal/state"
	"fmt"
	"time"
)

type MetricsService struct {
	scraper *metrics.Scraper
	state   *state.StateManager
	stop    chan struct{}
}

func NewMetricsService(st *state.StateManager) *MetricsService {
	return &MetricsService{
		scraper: metrics.NewScraper(),
		state:   st,
		stop:    make(chan struct{}),
	}
}

func (s *MetricsService) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		fmt.Println("📊 Metrics Collector started")
		for {
			select {
			case <-ticker.C:
				s.collectAll()
			case <-s.stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *MetricsService) Stop() {
	close(s.stop)
}

func (s *MetricsService) collectAll() {
	// Get IDs of running nodes
	nodeIDs := s.state.GetActiveNodes()

	for _, id := range nodeIDs {
		metrics, err := s.scraper.Collect(id)
		if err != nil {
			// Node might have stopped between check and collection
			continue
		}

		// Update State IN MEMORY
		// We use a dedicated method to update just metrics to avoid full lock contention if possible,
		// but for now, simple UpdateNode is fine.
		node, exists := s.state.GetNode(id)
		if exists {
			node.LatestMetrics = *metrics
			s.state.AddNode(*node) // Save back to memory
		}
	}
	// Note: We do NOT call s.state.Save() to disk here every 5 seconds.
	// That would kill the SSD. We keep metrics in RAM.
}
