// SPDX-License-Identifier: BSD-3-Clause
//go:build !aix && !solaris

package process

import "context"

func (p *Process) nativePercentWithContext(_ context.Context) (float64, bool, error) {
	return 0, false, nil
}
