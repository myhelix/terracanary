package stacks

import (
	"github.com/myhelix/terraform-experimental/terracanary/canarrors"
	"github.com/myhelix/terraform-experimental/terracanary/config"

	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var runningCmd *terraformCmd
var cmdMutex sync.Mutex

type Command struct {
	Stack
	Args             []string  // args for command
	Action           string    // terraform action name
	UseApplyArgs     bool      // Adds the args configured by "terracanary args" for plan/apply/destroy type commands
	Init             bool      // Perform terraform init before running command
	OutputSeparators bool      // Print big ====== boundaries around this command in output
	Interactive      bool      // Allow input from StdIn
	Stdout           io.Writer // If present, stdout will go here (otherwise, to stderr)
	Stderr           io.Writer // If present, stderr will go here (otherwise, to stderr)
	WorkingDirectory string    // Override normal working directory for Subdir
}

type terraformCmd struct {
	*exec.Cmd
}

func init() {
	handleSignals()

	os.Setenv("TF_IN_AUTOMATION", "true")
	//os.Setenv("TF_LOG", "TRACE")
}

// Keep track of what's running, so that the signal handler goroutine can kill it if necessary
func (c *terraformCmd) Run() (err error) {
	log.Println("Running in", c.Dir+":")
	log.Println(strings.Join(c.Args, " "))

	// Lock mutex while process is starting up to avoid signal handler race condition
	cmdMutex.Lock()
	runningCmd = c
	err = c.Cmd.Start()
	cmdMutex.Unlock()

	// If process started up, wait for it, with mutex unlocked to allow signal handling
	if err == nil {
		err = c.Cmd.Wait()
	}

	if err == nil {
		log.Println("Terraform exited success.")
	} else {
		log.Println("Terraform exited failure.")
	}

	// Locked mutex here both to synchronously unset runningCmd, but more importantly to ensure that if signal handling
	// is in process, our caller can't start a new command before signal handling completes and os.Exit is called.
	cmdMutex.Lock()
	runningCmd = nil
	cmdMutex.Unlock()
	return
}

// If we receive a request to exit (e.g. control-C), make sure we don't exit before terraform does; leaving
// terraform running in the background is confusing (and maybe dangerous without state-file locking)
func handleSignals() {
	c := make(chan os.Signal, 2) // Overflow signals will be dropped; we care about max of 2 signals
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-c
		cmdMutex.Lock() // We'll hold this until we exit to prevent new commands from starting
		if runningCmd != nil {
			// terraform will receive the signal directly; the shell or init process will signal our entire process group
			log.Println("Received first signal; waiting to see if terraform exits cleanly. Signal again to kill.")
			p := make(chan bool, 1)
			go func() {
				runningCmd.Process.Wait()
				p <- true
			}()
			select {
			case <-p:
				// Terraform exited on its own; fall through and exit.
			case sig := <-c:
				log.Println("Received 2nd signal; killing terraform.")
				// Give it a moment to process the 2nd signal itself before killing it outright
				time.Sleep(time.Millisecond * 500)
				runningCmd.Process.Kill()
				canarrors.Killed.Details(sig).Exit()
			}
		}
		canarrors.Interrupted.Details(sig).Exit()
	}()
}

func (s Stack) destroyPre() error {
	exists, err := s.Exists()
	if err != nil {
		return err
	}
	if !exists {
		return canarrors.NoSuchStack.Details(s)
	}
	return nil
}

func (s Stack) destroyPost() error {
	// Sometimes terraform thinks it failed when there's really nothing left. So we ignore the
	// terraform exit status, and check whether any resources actually remain in the state file.
	remaining, err := s.StateList()
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		log.Println("Remaining resources: ", remaining)
		return canarrors.IncompleteDestruction.Details("stack: ", s, " remaining: ", remaining)
	}
	// Successfully destroyed everything; clean up empty state file, so that future runs know this
	// stack no longer exists.
	err = s.RemoveState()
	if err != nil {
		return err
	}
	log.Println("Stack destroyed:", s)
	return nil
}

// Run normal destroy; no confirmation
func (s Stack) Destroy(inputStacks []Stack, additionalArgs ...string) error {
	if err := s.destroyPre(); err != nil {
		return err
	}
	additionalArgs = append(additionalArgs, "-force")
	s.RunAction("destroy", inputStacks, additionalArgs...)
	return s.destroyPost()
}

// There are situations where "terraform destroy" will fail because of missing data; we can bypass
// that by providing an empty-except-providers config. No confirmation.
// !!! THIS WILL OVERRIDE PREVENT-DESTROY !!!
func (s Stack) ForceDestroy(providerDefinitions string) error {
	if err := s.destroyPre(); err != nil {
		return err
	}

	log.Println("Attempting to force destruction using blank config.")

	destroyPlayground, err := ioutil.TempDir("", "terracanary-destroy")
	if err != nil {
		return err
	}
	defer os.RemoveAll(destroyPlayground) // clean up

	// We need a basic config with provider definitions to accomplish our destruction
	err = exec.Command("cp", providerDefinitions, destroyPlayground).Run()
	if err != nil {
		return err
	}

	Command{
		Stack:            s,
		WorkingDirectory: destroyPlayground,
		Init:             true,
		Action:           "destroy",
		Args:             []string{"-force"},
		UseApplyArgs:     true,
	}.Run()
	return s.destroyPost()
}

func (s Stack) RunAction(action string, inputStacks []Stack, additionalArgs ...string) error {
	cmd, err := s.ActionCommand(action, inputStacks, additionalArgs...)
	if err != nil {
		return err
	}
	return cmd.Run()
}

func (s Stack) RunInteractiveAction(action string, inputStacks []Stack, additionalArgs ...string) error {
	cmd, err := s.ActionCommand(action, inputStacks, additionalArgs...)
	if err != nil {
		return err
	}
	cmd.Interactive = true
	return cmd.Run()
}

func (s Stack) ActionCommand(action string, inputStacks []Stack, additionalArgs ...string) (*Command, error) {
	args := []string{}
	if s.Version != 0 {
		args = append(args, "-var", fmt.Sprintf("%s=%d",
			config.Global.StackVersionInput,
			s.Version,
		))
	}
	for _, stack := range inputStacks {
		varPrefix := stack.Subdir
		if stack.InputAlias != "" {
			varPrefix = stack.InputAlias
		}
		// Pass in info to find state file for input stack
		args = append(args, "-var", fmt.Sprintf(
			`%s={bucket="%s" key="%s" region="%s"}`,
			varPrefix+config.Global.StateInputPostfix,
			config.Global.StateFileBucket,
			stack.stateFileName(),
			config.Global.AWSRegion,
		), "-var", fmt.Sprintf(`%s=%d`,
			varPrefix+config.Global.StateVersionPostfix,
			stack.Version,
		))
	}
	args = append(args, additionalArgs...)

	return &Command{
		Stack:            s,
		Action:           action,
		Args:             args,
		UseApplyArgs:     true,
		Init:             true,
		OutputSeparators: true,
	}, nil
}

func (s Stack) StateList() (state []string, err error) {
	out, err := s.CmdOutput("state", "list")
	if err != nil {
		return
	}
	lines := strings.Split(out, "\n")
	for _, s := range lines {
		s = strings.TrimSpace(s)
		if s != "" {
			state = append(state, s)
		}
	}
	return
}

// This gets a single terraform output variable
func (s Stack) Output(name string) (string, error) {
	return s.CmdOutput("output", name)
}

// Get multiple terraform output variables, in the order specified
func (s Stack) Outputs(names ...string) (out []string, err error) {
	for _, name := range names {
		o, err := s.Output(name)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return
}

// This gets the entire output of an arbitrary terraform command
func (s Stack) CmdOutput(action string, args ...string) (string, error) {
	buf := bytes.Buffer{}
	cmd := Command{
		Stack:  s,
		Init:   true,
		Stdout: bufio.NewWriter(&buf),
		Action: action,
		Args:   args,
	}
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

type initState struct {
	Done    bool
	Version uint
}

var initStates = make(map[string]initState)

func (c Command) InitTerraform() error {
	// Skip cache if working directory is overridden
	if c.WorkingDirectory == "" {
		if initStates[c.Subdir].Done && initStates[c.Subdir].Version == c.Version {
			log.Printf("Already initialized %s to version %d.\n", c.Subdir, c.Version)
			return nil
		}
	}

	// Clear any existing local state; otherwise terraform will want to know if we should re-use it if there's no
	// existing remote state
	var oldstate string
	if c.WorkingDirectory == "" {
		oldstate = c.Subdir + "/.terraform/terraform.tfstate"
	} else {
		oldstate = c.WorkingDirectory + "/.terraform/terraform.tfstate"
	}

	err := os.Remove(oldstate)
	if err != nil {
		if !strings.Contains(err.Error(), "no such file") {
			return err
		}
		//log.Println("No existing state to remove at: " + oldstate)
	} else {
		//log.Println("Removed old state at: " + oldstate)
	}

	args := append(
		config.Global.InitArgs,
		"-backend-config=region="+config.Global.AWSRegion,
		"-backend-config=bucket="+config.Global.StateFileBucket,
		"-backend-config=key="+c.stateFileName(),
	)

	// Output isn't helpful unless there's some sort of failure
	buf := bytes.Buffer{}
	writer := bufio.NewWriter(&buf)

	cmd := Command{
		Stack:            c.Stack,
		Action:           "init",
		Args:             args,
		WorkingDirectory: c.WorkingDirectory,
		Stdout:           writer,
		Stderr:           writer,
	}

	err = cmd.Run()
	if err != nil {
		os.Stderr.Write(buf.Bytes())
		log.Println("\nFailed to initial terraform:", err.Error())
		return err
	}

	log.Printf("Initialized %s to version %d.\n", c.Subdir, c.Version)

	// Skip cache if working directory is overridden
	if c.WorkingDirectory == "" {
		initStates[c.Subdir] = initState{
			Done:    true,
			Version: c.Version,
		}
	}
	return nil
}

func (c Command) Run() error {
	if c.OutputSeparators {
		fmt.Fprintln(os.Stderr, "\n======================================================================\n")
	}

	if c.Init {
		err := c.InitTerraform()
		if err != nil {
			return err
		}
	}

	if c.Action == "" {
		panic("Action must be set")
	}

	args := []string{c.Action}
	if !c.Interactive {
		switch c.Action {
		case "init", "plan", "apply", "destroy":
			args = append(args, "-input=false")
		}
	}

	if c.UseApplyArgs {
		args = append(args, config.Global.TerraformArgs...)
	}

	args = append(args, c.Args...)

	cmd := exec.Command("terraform", args...)

	if c.WorkingDirectory == "" {
		// Normal case; based on Subdir
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		cmd.Dir = filepath.Join(wd, c.Subdir)
	} else {
		// Weird case; override working directory
		cmd.Dir = c.WorkingDirectory
	}

	// By default, send all output to stderr to avoid polluting output of terracanary commands with output data
	if c.Stdout == nil {
		cmd.Stdout = os.Stderr
	} else {
		cmd.Stdout = c.Stdout
	}
	if c.Stderr == nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = c.Stderr
	}
	if c.Interactive {
		cmd.Stdin = os.Stdin
	}

	err := (&terraformCmd{cmd}).Run()

	if c.OutputSeparators {
		fmt.Fprintln(os.Stderr, "\n======================================================================\n")
	}

	return err
}
