// SPDX-License-Identifier: BSD-3-Clause
//go:build aix || solaris

package process

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// nativePercentWithContext reads %CPU from ps -o pcpu on AIX and Solaris.
// ps -o time on these platforms has only 1-second resolution, so the delta
// approach in PercentWithContext always returns 0 over short intervals.
// ps -o pcpu reports per-total-system percentage (POSIX); multiply by NumCPU
// to match gopsutil's convention where 100 = one fully saturated core.
func (p *Process) nativePercentWithContext(ctx context.Context) (float64, bool, error) {
	out, err := invoke.CommandWithContext(ctx, "ps", "-o", "pcpu", "-p", fmt.Sprintf("%d", p.Pid))
	if err != nil {
		return 0, true, fmt.Errorf("ps pcpu failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, true, fmt.Errorf("unexpected ps output: %s", string(out))
	}
	val := strings.TrimSpace(lines[len(lines)-1])
	pct, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, true, fmt.Errorf("failed to parse pcpu %q: %w", val, err)
	}
	return pct * float64(runtime.NumCPU()), true, nil
}
