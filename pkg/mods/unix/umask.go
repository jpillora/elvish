//go:build unix

package unix

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/jpillora/elvish/pkg/eval/errs"
	"github.com/jpillora/elvish/pkg/eval/vals"
	"github.com/jpillora/elvish/pkg/eval/vars"
)

const (
	validUmaskMsg = "integer in the range [0..0o777]"
)

// UmaskVariable is a variable whose value always reflects the current file
// creation permission mask. Setting it changes the current file creation
// permission mask for the process (not an individual thread).
type UmaskVariable struct{}

var _ vars.Var = UmaskVariable{}

// There is no way to query the current umask without changing it, so we store
// the umask value in a variable, and initialize it on first use by a standalone shell. It needs to
// be mutex-guarded since it could be read or written concurrently.
//
// This assumes no other part of the Elvish code base involved in the
// interpreter ever calls unix.Umask, which is guaranteed by the
// check-content.sh script.
var (
	umaskVal   int
	umaskMutex sync.RWMutex
	umaskInit  sync.Once
)

func initializeUmask() {
	// Embedded sessions do not expose this process-wide setting. Importing
	// the package must never temporarily change the embedding process umask.
	umask := unix.Umask(0o777)
	unix.Umask(umask)
	umaskVal = umask
}

// Get returns the current file creation umask as a string.
func (UmaskVariable) Get() any {
	umaskInit.Do(initializeUmask)
	umaskMutex.RLock()
	defer umaskMutex.RUnlock()
	return fmt.Sprintf("0o%03o", umaskVal)
}

// Set changes the current file creation umask. It can be called with a string
// or a number. Strings are treated as octal numbers by default, unless they
// have an explicit base prefix like 0x or 0b.
func (UmaskVariable) Set(v any) error {
	umaskInit.Do(initializeUmask)
	umask, err := parseUmask(v)
	if err != nil {
		return err
	}

	umaskMutex.Lock()
	defer umaskMutex.Unlock()
	unix.Umask(umask)
	umaskVal = umask
	return nil
}

func parseUmask(v any) (int, error) {
	var umask int

	switch v := v.(type) {
	case string:
		i, err := strconv.ParseInt(v, 8, 0)
		if err != nil {
			i, err = strconv.ParseInt(v, 0, 0)
			if err != nil {
				return -1, errs.BadValue{
					What: "umask", Valid: validUmaskMsg, Actual: vals.ToString(v)}
			}
		}
		umask = int(i)
	case int:
		umask = v
	case float64:
		intPart, fracPart := math.Modf(v)
		if fracPart != 0 {
			return -1, errs.BadValue{
				What: "umask", Valid: validUmaskMsg, Actual: vals.ToString(v)}
		}
		umask = int(intPart)
	case *big.Int:
		return -1, errs.OutOfRange{
			What: "umask", ValidLow: "0", ValidHigh: "0o777",
			Actual: vals.ToString(v)}
	case *big.Rat:
		return -1, errs.BadValue{
			What: "umask", Valid: validUmaskMsg, Actual: vals.ToString(v)}
	default:
		return -1, errs.BadValue{
			What: "umask", Valid: validUmaskMsg, Actual: vals.Kind(v)}
	}

	if umask < 0 || umask > 0o777 {
		return -1, errs.OutOfRange{
			What: "umask", ValidLow: "0", ValidHigh: "0o777",
			Actual: fmt.Sprintf("%O", umask)}
	}
	return umask, nil
}
