// Copyright (c) The Tellor Authors.
// Licensed under the MIT License.

package pow

import (
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"github.com/tellor-io/telliot/pkg/config"
)

func SetupMiningGroup(cfg *config.Config, logger log.Logger, exitCh chan os.Signal) (*MiningGroup, error) {
	var hashers []Hasher
	gpus, err := GetOpenCLGPUs()
	level.Info(logger).Log("msg", "found gpus", "amount", len(gpus))
	if err != nil {
		return nil, err
	}
	for _, gpu := range gpus {
		gpuConfig, ok := cfg.GPUConfig[gpu.Name()]
		if !ok {
			gpuConfig = cfg.GPUConfig["default"]
		}
		if gpuConfig != nil && gpuConfig.Disabled {
			level.Info(logger).Log("msg", "gpu disabled in config ignored", "name", gpu.Name())
			continue
		}
		thisMiner, err := NewGpuMiner(gpu, gpuConfig, cfg.EnablePoolWorker)
		if err != nil {
			return nil, errors.Wrapf(err, "initializing GPU %s", gpu.Name())
		}
		hashers = append(hashers, thisMiner)
		level.Info(logger).Log(
			"msg", "check miner information",
			"name", thisMiner.Name(),
			"groupSize", thisMiner.GroupSize,
			"groups", thisMiner.Groups,
			"count", thisMiner.Count,
		)
	}
	if len(hashers) == 0 {
		level.Info(logger).Log(
			"msg", "no GPUs enabled, falling back to CPU mining",
			"numProcessors", cfg.NumProcessors,
		)
		for i := 0; i < cfg.NumProcessors; i++ {
			hashers = append(hashers, NewCpuMiner(int64(i)))
		}

	}
	return NewMiningGroup(hashers, exitCh), nil
}
