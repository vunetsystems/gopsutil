// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package process

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockInvoker struct {
	outputs map[string]string
}

func (m *mockInvoker) Command(name string, arg ...string) ([]byte, error) {
	return m.CommandWithContext(context.Background(), name, arg...)
}

func (m *mockInvoker) CommandWithContext(ctx context.Context, name string, arg ...string) ([]byte, error) {
	key := name
	for _, a := range arg {
		key += " " + a
	}
	if out, ok := m.outputs[key]; ok {
		return []byte(out), nil
	}
	return nil, assert.AnError
}

// An AIX test binary would also contain common tests
// which will call inimplemented aix functions,
// so after building the test binary, you must
// use `TestProcess_AIX` prefix for running AIX tests.
func TestProcess_AIX_MemoryInfo(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o rssize -p 1234": "RSS\n 1024\n",
			"ps -o vsz -p 1234":    "VSZ\n 2048\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	m, err := p.MemoryInfo()
	require.NoError(t, err)
	assert.Equal(t, uint64(1024*1024), m.RSS)
	assert.Equal(t, uint64(2048*1024), m.VMS)
}

func TestProcess_AIX_Times(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o time -p 1234": "TIME\n 01:02:03\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	times, err := p.Times()
	assert.NoError(t, err)
	assert.Equal(t, &cpu.TimesStat{User: 3723.0}, times)
}

func TestProcess_AIX_CreateTime(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o etimes -p 1234": "ELAPSED\n 3600\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	ctime, err := p.CreateTime()
	assert.NoError(t, err)
	assert.True(t, ctime > 0)
	// Check if it's roughly 1 hour ago (mock etimes is 3600)
	now := time.Now().Unix()
	expected := (now - 3600) * 1000
	assert.InDelta(t, expected, ctime, 3000)
}

func TestProcess_AIX_Cwd(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	p := &Process{Pid: 1234}
	pidDir := filepath.Join(td, "1234")
	err := os.MkdirAll(pidDir, 0755)
	assert.NoError(t, err)

	cwdPath := filepath.Join(pidDir, "cwd")
	target := filepath.Join(td, "target")
	err = os.MkdirAll(target, 0755)
	assert.NoError(t, err)

	err = os.Symlink(target, cwdPath)
	assert.NoError(t, err)

	cwd, err := p.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, target, cwd)
}

func TestParsePsTime(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"01:02:03", 3723},
		{"02:03", 123},
		{"1-01:02:03", 86400 + 3723},
	}

	for _, tt := range tests {
		got, err := parsePsTime(tt.input)
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, got)
	}
}

func TestProcess_AIX_Name(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o comm -p 1234": "COMMAND\n myproc\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	name, err := p.Name()
	require.NoError(t, err)
	assert.Equal(t, "myproc", name)
}

func TestProcess_AIX_Cmdline(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o args -p 1234": "COMMAND\n /usr/bin/myproc arg1 arg2\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	cmdline, err := p.Cmdline()
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/myproc arg1 arg2", cmdline)
}

func TestProcess_AIX_CmdlineSlice(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o args -p 1234": "COMMAND\n /usr/bin/myproc arg1 arg2\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	args, err := p.CmdlineSlice()
	require.NoError(t, err)
	assert.Equal(t, []string{"/usr/bin/myproc", "arg1", "arg2"}, args)
}

func TestProcess_AIX_Uids(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o uid -p 1234": "UID\n 1001\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	uids, err := p.Uids()
	require.NoError(t, err)
	assert.Equal(t, []uint32{1001}, uids)
}

func TestProcess_AIX_MemoryInfo_RssFallback(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	// rssize not available — falls back to rss
	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o rss -p 1234": "RSS\n 512\n",
			"ps -o vsz -p 1234": "VSZ\n 4096\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	m, err := p.MemoryInfo()
	require.NoError(t, err)
	assert.Equal(t, uint64(512*1024), m.RSS)
	assert.Equal(t, uint64(4096*1024), m.VMS)
}

func TestProcess_AIX_MemoryInfo_VsizeFallback(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	// vsz not available — falls back to vsize
	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o rssize -p 1234": "RSS\n 1024\n",
			"ps -o vsize -p 1234":  "VSIZE\n 2048\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	m, err := p.MemoryInfo()
	require.NoError(t, err)
	assert.Equal(t, uint64(1024*1024), m.RSS)
	assert.Equal(t, uint64(2048*1024), m.VMS)
}

func TestReadPidsFromDirAix(t *testing.T) {
	td := t.TempDir()
	for _, name := range []string{"1", "123", "456", "notapid", "7extra"} {
		require.NoError(t, os.MkdirAll(filepath.Join(td, name), 0755))
	}
	pids, err := readPidsFromDirAix(td)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int32{1, 123, 456}, pids)
}

func TestProcess_AIX_NativePercent(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o pcpu -p 1234": "%CPU\n  2.5\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	pct, ok, err := p.nativePercentWithContext(context.Background())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 2.5*float64(runtime.NumCPU()), pct)
}

func TestProcess_AIX_NativePercent_ZeroCPU(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o pcpu -p 1234": "%CPU\n  0.0\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	pct, ok, err := p.nativePercentWithContext(context.Background())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 0.0, pct)
}

func TestProcess_AIX_NativePercent_PsError(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	invoke = &mockInvoker{outputs: map[string]string{}}

	p := &Process{Pid: 9999}
	_, ok, err := p.nativePercentWithContext(context.Background())
	assert.True(t, ok) // ok=true even on error; caller should not fall back
	assert.Error(t, err)
}

func TestProcess_AIX_NativePercent_InvalidValue(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	mock := &mockInvoker{
		outputs: map[string]string{
			"ps -o pcpu -p 1234": "%CPU\n  notanumber\n",
		},
	}
	invoke = mock

	p := &Process{Pid: 1234}
	_, ok, err := p.nativePercentWithContext(context.Background())
	assert.True(t, ok)
	assert.Error(t, err)
}
