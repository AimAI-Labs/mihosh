package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "parameter error uses exit code 2",
			err:  wrapParameterError(errors.New("bad args")),
			want: exitCodeParameter,
		},
		{
			name: "config error uses exit code 3",
			err:  wrapConfigError(errors.New("config broken")),
			want: exitCodeConfig,
		},
		{
			name: "network error uses exit code 4",
			err:  wrapNetworkError(errors.New("dial tcp timeout")),
			want: exitCodeNetwork,
		},
		{
			name: "cobra parse error inferred as parameter error",
			err:  errors.New("unknown flag: --bad"),
			want: exitCodeParameter,
		},
		{
			name: "default error uses exit code 1",
			err:  errors.New("boom"),
			want: exitCodeGeneral,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := exitCodeForError(tc.err); got != tc.want {
				t.Fatalf("expected exit code %d, got %d", tc.want, got)
			}
		})
	}
}

func TestRenderCommandError(t *testing.T) {
	t.Parallel()

	i18n.Init()

	tests := []struct {
		name        string
		err         error
		wantPrefix  string
		wantContain string
	}{
		{
			name:        "parameter error message",
			err:         wrapParameterError(errors.New("参数不合法")),
			wantPrefix:  i18n.T("cli.errors.param_error"),
		},
		{
			name:       "config error message",
			err:        wrapConfigError(errors.New("配置文件不存在")),
			wantPrefix: i18n.T("cli.errors.config_error"),
		},
		{
			name:        "network error message",
			err:         wrapNetworkError(errors.New("dial tcp timeout")),
			wantPrefix:  i18n.T("cli.errors.network_error"),
		},
		{
			name:       "general error message",
			err:        errors.New("unknown failure"),
			wantPrefix: i18n.T("cli.errors.general_error"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderCommandError(tc.err)
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("expected prefix %q, got %q", tc.wantPrefix, got)
			}
			if tc.wantContain != "" && !strings.Contains(got, tc.wantContain) {
				t.Fatalf("expected message to contain %q, got %q", tc.wantContain, got)
			}
		})
	}
}
