/*
Staticlint runs different static (linter) checks on the project.

Static lint uses multilint to run different linters.

Usage:

	gofmt [flags] package

The flags are identical to multilint flags.

The exact checkers are:

	asmdecl.Analyzer -- reports mismatches between assembly files and Go declarations
	assign.Analyzer -- detects useless assignments
	atomic.Analyzer -- checks for common mistakes using the sync/atomic package
	atomicalign.Analyzer -- checks for non-64-bit-aligned arguments to sync/atomic functions
	bools.Analyzer -- detects common mistakes involving boolean operators
	buildtag.Analyzer -- checks build tags
	cgocall.Analyzer -- detects some violations of the cgo pointer passing rules
	composite.Analyzer -- checks for unkeyed composite literals
	copylock.Analyzer -- checks for locks erroneously passed by value
	deepequalerrors.Analyzer -- checks for the use of reflect.DeepEqual with error values
	errorsas.Analyzer -- checks that the second argument to errors.As is a pointer to a type implementing error
	fieldalignment.Analyzer -- detects structs that would use less memory if their fields were sorted
	framepointer.Analyzer -- reports assembly code that clobbers the frame pointer before saving it
	httpresponse.Analyzer -- checks for mistakes using HTTP responses
	ifaceassert.Analyzer -- flags impossible interface-interface type assertions
	loopclosure.Analyzer -- checks for references to enclosing loop variables from within nested functions
	lostcancel.Analyzer -- checks for failure to call a context cancellation function
	nilfunc.Analyzer -- checks for useless comparisons against nil
	printf.Analyzer -- hecks consistency of Printf format strings and arguments
	reflectvaluecompare.Analyzer -- checks for accidentally using == or reflect.DeepEqual to compare reflect.Value values
	shadow.Analyzer -- checks for shadowed variables
	shift.Analyzer -- checks for shifts that exceed the width of an integer
	sigchanyzer.Analyzer -- detects misuse of unbuffered signal as argument to signal.Notify
	sortslice.Analyzer -- checks for calls to sort.Slice that do not use a slice type as first argument
	stdmethods.Analyzer -- checks for misspellings in the signatures of methods similar to well-known interfaces
	stringintconv.Analyzer -- flags type conversions from integers to strings
	structtag.Analyzer -- checks struct field tags are well formed
	unmarshal.Analyzer -- checks for passing non-pointer or non-interface types to unmarshal and decode functions
	unreachable.Analyzer -- checks for unreachable code
	unsafeptr.Analyzer -- checks for invalid conversions of uintptr to unsafe.Pointer
	unusedresult.Analyzer -- checks for unused results of calls to certain pure functions
	unusedwrite.Analyzer -- checks for unused writes to the elements of a struct or array object
	usesgenerics.Analyzer -- checks for usage of generic features

and also custom analyzer MainExitAnalyzer which flags all calls to os.Exit in the function main
of the package main
*/
package main

import (
	"logogger/internal/linter"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/usesgenerics"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

// Timespan in which key has to be refreshed prior to its spoilage
func main() {
	var analyzers []*analysis.Analyzer

	analyzers = append(
		analyzers,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		atomicalign.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		deepequalerrors.Analyzer,
		errorsas.Analyzer,
		fieldalignment.Analyzer,
		framepointer.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		reflectvaluecompare.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		sortslice.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		unusedwrite.Analyzer,
		usesgenerics.Analyzer,
	)

	for _, v := range simple.Analyzers {
		analyzers = append(analyzers, v.Analyzer)
	}
	for _, v := range staticcheck.Analyzers {
		analyzers = append(analyzers, v.Analyzer)
	}
	for _, v := range stylecheck.Analyzers {
		analyzers = append(analyzers, v.Analyzer)
	}

	analyzers = append(analyzers, linter.MainExitAnalyzer)

	multichecker.Main(analyzers...)
}
