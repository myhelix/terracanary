package canarrors

import (
	"errors"
	"fmt"
	"log"
	"os"
)

var (
	InvalidStack          = ErrorType{11, "Invalid stack"}
	OddStackSelection     = ErrorType{12, "Are you sure that's the stack you want?"}
	IncompleteDestruction = ErrorType{13, "Some resources left over after destroy"}
	NoSuchStack           = ErrorType{14, "Stack does not exist"}
	PlanHasChanges        = ErrorType{15, "Tested plan has changes"}
	Interrupted           = ErrorType{16, "Exited cleanly; interrupted by signal"}
	Killed                = ErrorType{17, "Killed terraform; interrupted by signal"}
)

type ErrorType struct {
	ExitCode    int
	Description string
}

type Error struct {
	ErrorType
	error
}

func (t ErrorType) Details(stuff ...interface{}) Error {
	return Error{t, errors.New(fmt.Sprint(stuff...))}
}

func (t ErrorType) With(err error) Error {
	return Error{t, err}
}

func (err Error) Error() string {
	return err.Description + ": " + err.error.Error()
}

func (err Error) Exit() {
	ExitWith(err)
}

func ExitWith(err error) {
	log.Println("Exited due to error:")
	log.Println("\t" + err.Error())
	exitCode := 1
	if err, ok := err.(Error); ok {
		exitCode = err.ExitCode
	}
	os.Exit(exitCode)
}

func ExitIf(err error) {
	if err != nil {
		ExitWith(err)
	}
}

func Is(err error, t ErrorType) bool {
	ce, ok := err.(Error)
	return ok && ce.ErrorType == t
}
