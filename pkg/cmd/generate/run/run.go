package run

import (
	kcl "kcl-lang.io/kcl-go"
	kclpkg "kcl-lang.io/kcl-go/pkg/kcl"
	"kcl-lang.io/kpm/pkg/api"
	"kcl-lang.io/kpm/pkg/opt"
)

// CodeRunner compiles and runs the target DSL based configuration code
// and returns configuration data in plain format.
type CodeRunner interface {
	Run(workingDir string, arguments map[string]string) ([]byte, error)
}

// KPMRunner should implement the CodeRunner interface.
var _ CodeRunner = &KPMRunner{}

// KPMRunner implements the CodeRunner interface.
type KPMRunner struct{}

// Run calls KPM api to compile and run KCL based configuration code.
func (r *KPMRunner) Run(workDir string, arguments map[string]string) ([]byte, error) {
	optList := buildKCLOptions(workDir, arguments)
	result, err := api.RunWithOpts(
		opt.WithKclOption(*kclpkg.NewOption().Merge(optList...)),
		opt.WithNoSumCheck(true),
		opt.WithLogWriter(nil),
	)
	if err != nil {
		return nil, err
	}

	return []byte(result.GetRawYamlResult()), nil
}

// buildKCLOptions returns list of KCL options.
func buildKCLOptions(workDir string, arguments map[string]string) []kcl.Option {
	optList := make([]kcl.Option, 2)

	// build arguments option
	for k, v := range arguments {
		argStr := k + "=" + v
		withOpt := kcl.WithOptions(argStr)
		optList = append(optList, withOpt)
	}

	// build workDir option
	withOpt := kcl.WithWorkDir(workDir)
	optList = append(optList, withOpt)

	// eliminate null values in the result
	withOpt = kcl.WithDisableNone(true)
	optList = append(optList, withOpt)

	return optList
}
