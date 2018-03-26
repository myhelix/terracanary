package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Config struct {
	// Set by 'terracanary init'
	AWSRegion       string
	StateFileBase   string
	StateFileBucket string
	InitArgs        []string

	// Set by 'terracanary args'
	TerraformArgs []string

	// TODO: Provide way of setting these
	StateInputPostfix   string
	StateVersionPostfix string
	StackVersionInput   string
}

var Global *Config

func Initialize() {
	Global = &Config{
		StateInputPostfix:   "_stack_state",
		StateVersionPostfix: "_stack_version",
		StackVersionInput:   "stack_version",
	}
}

const configFile = ".terracanary"

func Read() error {
	jsn, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsn, &Global)
	if err != nil {
		return err
	}
	return nil
}

func Write() error {
	jsn, err := json.MarshalIndent(Global, "", "    ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(configFile, jsn, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
