// SPDX-License-Identifier: Apache-2.0

package profile

import (
	"os"
	"runtime/pprof"
)

var stopProfilingQueue = []func(){}

func StartProfiling(cpuFile, memoryFile string) error {
	cpuF, err := os.Create(cpuFile)
	if err != nil {
		return err
	}

	if err := pprof.StartCPUProfile(cpuF); err != nil {
		return err
	}

	stopProfilingQueue = append(stopProfilingQueue, func() {
		pprof.StopCPUProfile()
		cpuF.Close() //nolint:errcheck
	})

	memoryF, err := os.Create(memoryFile)
	if err != nil {
		return err
	}

	stopProfilingQueue = append(stopProfilingQueue, func() {
		pprof.WriteHeapProfile(memoryF) //nolint:errcheck
		memoryF.Close()                 //nolint:errcheck
	})

	return nil
}

func StopProfiling() {
	for _, f := range stopProfilingQueue {
		if f != nil {
			f()
		}
	}
}
