package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
)

type Application interface {
	BuildCmd() string

	BuildOutputDir() string

	RunCmd(port int) string

	StatusEndpoint() string

	// e.g. prod -> user@host  (no path)
	SshTarget(server string) string
}

type AppImpl struct {
	def ApplicationDef
}

type ApplicationDef struct {
	BuildCmd string

	BuildOutputDir string

	// needs a %PORT% part for port subsitution
	RunCmd string

	StatusEndpoint string

	// e.g. user@host  (no path)
	SshTarget map[string]string
}

func ApplicationFromConfig(file string) Application {
	var def ApplicationDef

	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalf("Failed to read deploy file: %s, error: %s", file, err)
	}

	json.Unmarshal(data, &def)

	return &AppImpl{def}
}

func (a *AppImpl) RunCmd(port int) string {
	return strings.Replace(a.def.RunCmd, "%PORT%", fmt.Sprintf("%d", port), -1)
}
func (a *AppImpl) SshTarget(server string) string {
	return a.def.SshTarget[server]
}

func (a *AppImpl) BuildCmd() string {
	return a.def.BuildCmd
}
func (a *AppImpl) BuildOutputDir() string {
	return a.def.BuildOutputDir
}
func (a *AppImpl) StatusEndpoint() string {
	return a.def.StatusEndpoint
}
