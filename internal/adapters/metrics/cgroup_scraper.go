package metrics

import (
	"Nexus/internal/core"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const CgroupRoot = "/sys/fs/cgroup"

type Scraper struct {
	prevCpu  map[string]int64 // in order to keep the previous cpu usage
	prevTime map[string]time.Time
}

// NewScraper creates a Scraper with initialized maps for tracking previous CPU usage and timestamps per NodeId.
// The returned Scraper is ready to collect metrics and compute CPU usage deltas across subsequent samples.
func NewScraper() *Scraper {
	return &Scraper{
		prevCpu:  make(map[string]int64),
		prevTime: make(map[string]time.Time),
	}
}

func (s *Scraper) Collect(NodeId string) (*core.NodeMetrics, error) {
	// The first step is to read the memory of the container from the cgroup folder
	memoryPath := filepath.Join(CgroupRoot, "memory/nexus", NodeId, "memory.usage_in_bytes")
	memoryBytes, ReadingMemoryBytesError := readInt64(memoryPath)
	if ReadingMemoryBytesError != nil {
		// This means the file doesn't exist, meaning the container/node is dead
		return nil, fmt.Errorf("unable to collect memory informations, container might be dead or wrong path, %v", ReadingMemoryBytesError)
	}
	// This meant we pass this step and then the file is existing and we can continue
	memoryLimitPath := filepath.Join(CgroupRoot, "memory/nexus", NodeId, "memory.limit_in_bytes")
	memoryLimitBytes, _ := readInt64(memoryLimitPath) // I went with a blank identifier because if the limit wasn't properly set , the container wouldn't have launched

	// The next step will be to read the cpu from the cgroup as we did
	cpuPath := filepath.Join(CgroupRoot, "cpu/nexus", NodeId, "cpuacct.usage")
	cpuBytes, ReadingCPUBytesError := readInt64(cpuPath)
	if ReadingCPUBytesError != nil {
		return nil, fmt.Errorf("unable to fetch the total cpu in use right now , %v", ReadingCPUBytesError)
	}

	// The then step is collect the current time
	now := time.Now()
	cpuPercent := 0.0

	if prevVal, ok := s.prevCpu[NodeId]; ok {
		prevT := s.prevTime[NodeId]

		// Delta in Nanoseconds
		deltaCPU := cpuBytes - prevVal
		deltaTime := now.Sub(prevT).Nanoseconds()

		if deltaTime > 0 {
			// Usage = (Delta CPU / Delta Time) * 100 * NumCPUs (but we treat it relative to 1 core here)
			cpuPercent = (float64(deltaCPU) / float64(deltaTime)) * 100.0
		}
	}

	s.prevCpu[NodeId] = cpuBytes
	s.prevTime[NodeId] = now

	return &core.NodeMetrics{
		NodeID:        NodeId,
		Timestamp:     now,
		MemoryUsage:   memoryBytes,
		MemoryLimit:   memoryLimitBytes,
		CPUPercent:    cpuPercent,
		CPUUsageTotal: cpuBytes,
	}, nil
}

// readInt64 reads the file at the given path, trims surrounding whitespace, and parses its contents as a base-10 int64.
// It returns the parsed integer or an error if the file cannot be read or the contents cannot be parsed.
func readInt64(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	str := strings.TrimSpace(string(data))
	return strconv.ParseInt(str, 10, 64)
}
