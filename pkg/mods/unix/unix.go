//go:build unix

// Package unix exports an Elvish namespace that contains variables and
// functions that deal with features unique to Unix-like operating systems. On
// non-Unix operating systems it exports an empty namespace.
package unix

import (
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/eval/vars"
	"github.com/jpillora/elvish/pkg/logutil"
)

// ExposeUnixNs indicate whether this module should be exposed as a usable
// elvish namespace.
const ExposeUnixNs = true

// Ns is an Elvish namespace that contains variables and functions that deal
// with features unique to Unix-like operating systems.
var Ns = eval.BuildNs().
	AddVars(map[string]vars.Var{
		"umask":   UmaskVariable{},
		"rlimits": rlimitsVar{},
	}).Ns()

var logger = logutil.GetLogger("[mods/unix] ")
