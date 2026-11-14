package complete

import (
	gocontext "context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpillora/elvish/pkg/eval"
)

func TestCompletionUsesVirtualDirectoryAndEnvironment(t *testing.T) {
	for _, name := range []string{"alpha", "beta"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, name+".txt"), nil, 0600)
			p, err := eval.NewProcess(gocontext.Background(), dir, []string{"HOME=" + dir, "SESSION_" + name + "=yes"})
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			ev := eval.NewEvalerWithProcess(p)
			for _, tc := range []struct{ code, want string }{{"echo ", name + ".txt "}, {"echo $E:SESSION_", "SESSION_" + name}} {
				result, err := Complete(CodeBuffer{Content: tc.code, Dot: len(tc.code)}, ev, Config{})
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, item := range result.Items {
					if strings.Contains(item.ToInsert, tc.want) {
						found = true
					}
				}
				if !found {
					t.Fatalf("completion %q did not contain %q: %#v", tc.code, tc.want, result.Items)
				}
			}
		})
	}
}
