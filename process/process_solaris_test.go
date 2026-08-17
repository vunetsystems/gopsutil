// SPDX-License-Identifier: BSD-3-Clause
//go:build solaris

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

func TestProcess(t *testing.T) {
	t.Run("TestProcess_Solaris_MemoryInfo", func(t *testing.T) {
		originalInvoke := invoke
		defer func() { invoke = originalInvoke }()

		mock := &mockInvoker{
			outputs: map[string]string{
				"ps -o rss -p 1234": "RSS\n 1024\n",
				"ps -o vsz -p 1234": "VSZ\n 2048\n",
			},
		}
		invoke = mock

		p := &Process{Pid: 1234}
		m, err := p.MemoryInfoWithContext(context.Background())
		require.NoError(t, err)
		assert.Equal(t, uint64(1024*1024), m.RSS)
		assert.Equal(t, uint64(2048*1024), m.VMS)
	})

	t.Run("TestProcess_Solaris_Times", func(t *testing.T) {
		originalInvoke := invoke
		defer func() { invoke = originalInvoke }()

		mock := &mockInvoker{
			outputs: map[string]string{
				"ps -o time -p 1234": "TIME\n 01:02:03\n",
			},
		}
		invoke = mock

		p := &Process{Pid: 1234}
		times, err := p.TimesWithContext(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, &cpu.TimesStat{User: 3723.0}, times)
	})

	t.Run("TestProcess_Solaris_CreateTime", func(t *testing.T) {
		originalInvoke := invoke
		defer func() { invoke = originalInvoke }()

		mock := &mockInvoker{
			outputs: map[string]string{
				"ps -o etime -p 1234": "ELAPSED\n 01:00:00\n",
			},
		}
		invoke = mock

		p := &Process{Pid: 1234}
		ctime, err := p.createTimeWithContext(context.Background())
		assert.NoError(t, err)
		assert.True(t, ctime > 0)
		// mock etime is 1 hour (3600 seconds)
		now := time.Now().Unix()
		expected := (now - 3600) * 1000
		assert.InDelta(t, expected, ctime, 3000)
	})

	t.Run("TestProcess_Solaris_Uids", func(t *testing.T) {
		originalInvoke := invoke
		defer func() { invoke = originalInvoke }()

		mock := &mockInvoker{
			outputs: map[string]string{
				"ps -o uid -p 1234": "UID\n 1001\n",
			},
		}
		invoke = mock

		p := &Process{Pid: 1234}
		uids, err := p.UidsWithContext(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []uint32{1001}, uids)
	})
}

func TestProcess_Solaris_Name(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	pidDir := filepath.Join(td, "1234")
	require.NoError(t, os.MkdirAll(pidDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "execname"), []byte("/usr/bin/myproc"), 0644))

	p := &Process{Pid: 1234}
	name, err := p.NameWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "myproc", name)
}

func TestProcess_Solaris_Exe(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	pathDir := filepath.Join(td, "1234", "path")
	require.NoError(t, os.MkdirAll(pathDir, 0755))
	require.NoError(t, os.Symlink("/usr/bin/myproc", filepath.Join(pathDir, "a.out")))

	p := &Process{Pid: 1234}
	exe, err := p.ExeWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/myproc", exe)
}

func TestProcess_Solaris_ExeFallback(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	// No path/a.out — should fall back to execname
	pidDir := filepath.Join(td, "1234")
	require.NoError(t, os.MkdirAll(pidDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "execname"), []byte("/usr/bin/myproc"), 0644))

	p := &Process{Pid: 1234}
	exe, err := p.ExeWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/myproc", exe)
}

func TestProcess_Solaris_Cmdline(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	pidDir := filepath.Join(td, "1234")
	require.NoError(t, os.MkdirAll(pidDir, 0755))
	// cmdline is null-separated
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("/usr/bin/myproc\x00arg1\x00arg2\x00"), 0644))

	p := &Process{Pid: 1234}
	cmdline, err := p.CmdlineWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/myproc arg1 arg2", cmdline)
}

func TestProcess_Solaris_CmdlineSlice(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	pidDir := filepath.Join(td, "1234")
	require.NoError(t, os.MkdirAll(pidDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("/usr/bin/myproc\x00arg1\x00arg2\x00"), 0644))

	p := &Process{Pid: 1234}
	args, err := p.CmdlineSliceWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"/usr/bin/myproc", "arg1", "arg2"}, args)
}

func TestProcess_Solaris_Cwd(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	pathDir := filepath.Join(td, "1234", "path")
	require.NoError(t, os.MkdirAll(pathDir, 0755))
	target := filepath.Join(td, "workdir")
	require.NoError(t, os.MkdirAll(target, 0755))
	require.NoError(t, os.Symlink(target, filepath.Join(pathDir, "cwd")))

	p := &Process{Pid: 1234}
	cwd, err := p.CwdWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, target, cwd)
}

func TestProcess_Solaris_NumFDs(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOST_PROC", td)

	fdDir := filepath.Join(td, "1234", "fd")
	require.NoError(t, os.MkdirAll(fdDir, 0755))
	for _, name := range []string{"0", "1", "2"} {
		f, err := os.Create(filepath.Join(fdDir, name))
		require.NoError(t, err)
		f.Close()
	}

	p := &Process{Pid: 1234}
	n, err := p.NumFDsWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(3), n)
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

func TestReadPidsFromDir(t *testing.T) {
	td := t.TempDir()
	for _, name := range []string{"1", "123", "456", "notapid", "self", "thread-self"} {
		require.NoError(t, os.MkdirAll(filepath.Join(td, name), 0755))
	}
	pids, err := readPidsFromDir(td)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int32{1, 123, 456}, pids)
}

func TestProcess_Solaris_NativePercent(t *testing.T) {
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

func TestProcess_Solaris_NativePercent_ZeroCPU(t *testing.T) {
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

func TestProcess_Solaris_NativePercent_PsError(t *testing.T) {
	originalInvoke := invoke
	defer func() { invoke = originalInvoke }()

	invoke = &mockInvoker{outputs: map[string]string{}}

	p := &Process{Pid: 9999}
	_, ok, err := p.nativePercentWithContext(context.Background())
	assert.True(t, ok) // ok=true even on error; caller should not fall back
	assert.Error(t, err)
}

func TestProcess_Solaris_NativePercent_InvalidValue(t *testing.T) {
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
