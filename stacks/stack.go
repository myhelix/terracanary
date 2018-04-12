package stacks

import (
	"github.com/myhelix/terracanary/canarrors"
	"github.com/myhelix/terracanary/config"

	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
)

var Legacy = Stack{legacy: true}

type Stack struct {
	Subdir           string
	Version          uint
	InputAlias       string // Used for input stacks to provide alternate input variable prefix
	WorkingDirectory string // Override normal working directory for Subdir
	legacy           bool   // Is this the legacy stack?
}

func New(subdir string, version uint) Stack {
	return Stack{
		Subdir:  subdir,
		Version: version,
	}
}

// Version string may be blank, meaning no version / 0
func Parse(subdir, vs string) (Stack, error) {
	if subdir == "" {
		return Stack{}, errors.New("Can't parse stack with no name.")
	}
	if vs == "" {
		return New(subdir, 0), nil
	}
	version, err := strconv.ParseUint(vs, 10, 32)
	if err != nil {
		return Stack{}, canarrors.InvalidStack.With(
			fmt.Errorf("Error parsing stack version '%s': %s", vs, err.Error()))
	}
	return New(subdir, uint(version)), nil
}

func (s Stack) String() string {
	if s.legacy {
		return "legacy"
	}
	str := s.Subdir
	if s.Version != 0 {
		str = fmt.Sprintf("%s:%d", str, s.Version)
	}
	return str

}

func (s Stack) GeneratePlan(inputStacks []Stack, additionalArgs ...string) (plan []byte, err error) {
	f, err := ioutil.TempFile("", "terracanary-plan")
	f.Close()
	defer func() {
		os.Remove(f.Name())
	}()

	additionalArgs = append(additionalArgs, "-out", f.Name())
	err = s.RunAction("plan", inputStacks, additionalArgs...)
	if err != nil {
		return
	}
	plan, err = ioutil.ReadFile(f.Name())
	return
}

func (s Stack) stateFileName() string {
	switch {
	case s.legacy:
		// This special case means a legacy statefile at the base path
		return config.Global.StateFileBase
	case s.Version == 0:
		// Non-versioned stack
		return s.stateFilePrefix()
	default:
		return fmt.Sprintf("%s-%d", s.stateFilePrefix(), s.Version)
	}
}

func (s Stack) stateFilePrefix() string {
	return fmt.Sprintf("%s-%s", config.Global.StateFileBase, s.Subdir)
}

// Used by aws.go to search for and parse stacks
func fromStateFileName(fileName string) (Stack, error) {
	re := regexp.MustCompile(fmt.Sprintf("^%s(-([a-z]+)(-([0-9]+))?)?$",
		regexp.QuoteMeta(config.Global.StateFileBase)),
	)
	/*
		re.FindStringSubmatch("foo-sfs-24")
			[]string{ "foo-sfs-24", "-sfs-24", "sfs", "-24", "24"}

		re.FindStringSubmatch("foo")
			[]string{ "foo", "", "", "", ""}
	*/

	groups := re.FindStringSubmatch(fileName)
	if groups == nil || len(groups) < 5 {
		return Stack{}, fmt.Errorf("Filename '%s' did not match pattern.", fileName)
	}

	if groups[2] == "" {
		return Legacy, nil
	}

	return Parse(groups[2], groups[4])
}

// Returns stacks in a that do not appear in b
func Subtract(a, b []Stack) (res []Stack) {
a:
	for _, i := range a {
		for _, j := range b {
			if i == j {
				continue a
			}
		}
		res = append(res, i)
	}
	return
}
