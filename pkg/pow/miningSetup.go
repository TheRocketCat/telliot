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
	level.Info(logger).Log("msg", "starting CPU mining", "threads", cfg.NumProcessors)
	for i := 0; i < cfg.NumProcessors; i++ {
		hashers = append(hashers, NewCpuMiner(int64(i)))
	}
	return NewMiningGroup(hashers, exitCh), nil
}
