package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

type Application interface {
	BuildCmd() string

	BuildOutputDir() string

	PostDeployCmd() string

	RunCmd(port int) string

	HealthEndpoint() string

	// e.g. prod -> Target{...}
	Target(server string) *Target
}

type AppImpl struct {
	def ApplicationDef
}

type Target struct {
	Ssh string // e.g. user@host

	SshPort int // optional

	Base int // camus base port, e.g. 8000
}

type ApplicationDef struct {
	Name string

	BuildCmd string

	BuildOutputDir string

	PostDeployCmd string

	// needs a %PORT% part for port subsitution
	RunCmd string

	HealthEndpoint string

	// e.g. user@host  (no path)
	Targets map[string]*Target
}

func ApplicationFromConfig(isClient bool, file string) (Application, error) {
	var def ApplicationDef

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	errMsg := func(str string, args ...interface{}) (Application, error) {
		return nil, fmt.Errorf("deploy.json: "+str, args...)
	}

	if err := json.Unmarshal(data, &def); err != nil {
		return errMsg(fmt.Sprintf("Invalid json %s", err))
	}

	if isClient {
		if len(def.Name) == 0 {
			return errMsg("Missing Name")
		}
		if len(def.BuildCmd) == 0 {
			return errMsg("Missing BuildCmd")
		}
		if len(def.BuildOutputDir) == 0 {
			return errMsg("Missing BuildOutputDir")
		}

		foundTarget := false
		for name, target := range def.Targets {
			foundTarget = true
			if len(target.Ssh) == 0 {
				return errMsg("Missing %s.Ssh", name)
			}
			if target.Base == 0 {
				return errMsg("Missing %s.Base", name)
			}
			if target.SshPort == 0 {
				target.SshPort = 22
			}
		}

		if !foundTarget {
			return errMsg("No targets specified")
		}
	}

	if len(def.RunCmd) == 0 {
		return errMsg("Missing RunCmd")
	}

	if len(def.HealthEndpoint) == 0 {
		if isClient {
			return errMsg("Missing HealthEndpoint")
		} else {
			def.HealthEndpoint = "/"
		}
	}

	return &AppImpl{def}, nil
}

func (a *AppImpl) RunCmd(port int) string {
	return strings.Replace(a.def.RunCmd, "%PORT%", fmt.Sprintf("%d", port), -1)
}
func (a *AppImpl) Target(server string) *Target {
	return a.def.Targets[server]
}

func (a *AppImpl) BuildCmd() string {
	return a.def.BuildCmd
}
func (a *AppImpl) PostDeployCmd() string {
	return a.def.PostDeployCmd
}
func (a *AppImpl) BuildOutputDir() string {
	return a.def.BuildOutputDir
}
func (a *AppImpl) HealthEndpoint() string {
	return a.def.HealthEndpoint
}
