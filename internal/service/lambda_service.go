package service

import (
	"Nexus/internal/adapters/runtime"
	"Nexus/internal/core"
	"Nexus/internal/ports"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type LambdaService struct {
	runtime ports.ContainerRuntime
}

// NewLambdaService creates a LambdaService backed by a libcontainer-based ContainerRuntime.
// It returns the initialized LambdaService or an error if the container runtime cannot be created.
func NewLambdaService() (*LambdaService, error) {
	rt, err := runtime.NewLibContainerRuntime()
	if err != nil {
		return nil, err
	}
	return &LambdaService{runtime: rt}, nil
}

type LambdaResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

func (s *LambdaService) Execute(code string, runtime string) (*LambdaResult, error) {
	// 1. Setup Identifier and Paths
	id := "lambda-" + uuid.New().String()
	hostTmpDir := "/tmp/nexus/lambdas"
	os.MkdirAll(hostTmpDir, 0755)

	hostScriptPath := filepath.Join(hostTmpDir, id+".py")

	// 2. Write Code to Host
	if err := os.WriteFile(hostScriptPath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write code: %w", err)
	}
	// CLEANUP: Delete script after execution
	defer os.Remove(hostScriptPath)

	// 3. Prepare Config
	// Note: VolumePath here is used to pass the SCRIPT path, intercepted by our adapter logic above
	conf := core.NodeConfig{
		ID:         id,
		Memory:     128,  // Lambdas are small
		CPUShares:  1024, // Give them full burst power
		RootfsPath: "/var/lib/nexus/images/alpine-base",
		Command:    []string{"python3", "/code/main.py"},
		VolumePath: hostScriptPath, // HACK: reusing this field to pass script location
	}

	var stdout, stderr bytes.Buffer
	start := time.Now()

	// 4. Run
	exitCode, err := s.runtime.RunEphemeral(conf, &stdout, &stderr)
	if err != nil {
		return nil, err
	}

	duration := time.Since(start)

	return &LambdaResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}
