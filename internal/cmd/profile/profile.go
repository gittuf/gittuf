// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package profile

import (
	"os"
	"runtime/pprof"
)

var stopProfilingQueue = []func() error{}

func StartProfiling(cpuFile, memoryFile string) error {
	cpuF, err := os.Create(cpuFile)
	if err != nil {
		return err
	}

	if err := pprof.StartCPUProfile(cpuF); err != nil {
		return err
	}

	stopProfilingQueue = append(stopProfilingQueue, func() error {
		pprof.StopCPUProfile()
		return cpuF.Close()
	})

	memoryF, err := os.Create(memoryFile)
	if err != nil {
		return err
	}

	stopProfilingQueue = append(stopProfilingQueue, func() error {
		if err := pprof.WriteHeapProfile(memoryF); err != nil {
			return err
		}
		return memoryF.Close()
	})

	return nil
}

func StopProfiling() error {
	for _, f := range stopProfilingQueue {
		if f != nil {
			if err := f(); err != nil {
				return err
			}
		}
	}

	return nil
}
