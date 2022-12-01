package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// const settingsFile = "unboundCollector.yml"

const (

	// The seetings file without a path is used in the dev environment
	settingsFile = "/var/db/newrelic-infra/custom-integrations/nri-unbound-config.yml"
	// settingsFile       = "nri-unbound-config.yml"
	// EntityType         = "UnboundDNSResolver"
	EntityType         = "APPLICATION"
	integrationName    = "nri-unbound"
	integrationVersion = "0.1.0"
)

var unboundControl = "unbound-control"

type settings struct {
	Instances struct {
		ControlPath string `yaml:"controlPath"`
		Cfgfile     string
		Server      string
		EntityName  string `yaml:"entityName"`
		DisplayName string `yaml:"displayName"`
		Debug       bool   `yaml:"debug,omitempty"`
		Reset       bool   `yaml:"reset,omitempty"`
		Mock        bool   `yaml:"mock,omitempty"`
		// Metadata    struct{} `yaml:"metadata"`
		Metadata []struct {
			Key   string `yaml:"key"`
			Value string `yaml:"value"`
		} `yaml:"metadata"`
	}
}

type metricsType struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Value      float64  `json:"value"`
	Attrinutes struct{} `json:"attributes"`
}

type entityType struct {
	Entity struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		DisplayName string `json:"displayName"`
		// Metadata    struct{} `json:"metadata"`
		Metadata map[string]string `json:"metadata"`
	}
	Metrics []metricsType
}

type Tags struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type payloadType struct {
	Protocol_version string `json:"protocol_version"`
	Integration      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	Data []entityType
}

func loadSettings(filename string) (*settings, error) {

	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open config file %q: %w", filename, err)
	}

	c := &settings{}
	err = yaml.Unmarshal(buf, c)

	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filename, err)
	}

	return c, err
}

func main() {
	/*
		load settings
		run unbound-control
		parse the data
		output properly formatted json
	*/

	// Load the settings

	settings, err := loadSettings(settingsFile)
	if err != nil {
		log.Fatal(err)
	}

	// Start to build the payload
	var payload payloadType
	payload.Protocol_version = "4"
	payload.Integration.Name = "nri-unbound"
	payload.Integration.Version = "1.1"

	var entityClass entityType
	entityClass.Entity.Name = settings.Instances.EntityName
	entityClass.Entity.Type = EntityType
	entityClass.Entity.DisplayName = settings.Instances.DisplayName

	// Add the metadata
	attribs := make(map[string]string)

	for _, s := range settings.Instances.Metadata {
		attribs[s.Key] = s.Value
	}
	entityClass.Entity.Metadata = attribs

	// Add the entity to the payload
	payload.Data = append(payload.Data, entityClass)

	// build the parameter list for the command
	var args string

	// TODO Add all other availabe command lne args for unbound-control

	args = "stats"
	if settings.Instances.Cfgfile != "" {
		args += " -c " + settings.Instances.Cfgfile
	}
	if settings.Instances.Server != "" {
		args += " -s " + settings.Instances.Server
	}

	// Run the command to collect metircs

	// If we are in a dev environment we can specify mock: true in the config.yml
	// to run the mock unbound-command
	if settings.Instances.Mock {
		unboundControl += "-mock"
	}

	// If a path to unbound-control has been specified add it to the command
	if settings.Instances.ControlPath != "" {
		unboundControl = settings.Instances.ControlPath + unboundControl
	}

	// Execute unbound-control
	cmdStruct := exec.Command(unboundControl, args)
	metrics, err := cmdStruct.Output()
	if err != nil {
		fmt.Println(err)
	}

	// parse the data
	scanner := bufio.NewScanner(strings.NewReader(string(metrics)))
	for scanner.Scan() {
		var metricValues = strings.Split(scanner.Text(), "=")
		var newMetric metricsType
		newMetric.Name = metricValues[0]
		newMetric.Type = "cumulative-count"
		newMetric.Value, err = strconv.ParseFloat(metricValues[1], 64)
		if err != nil {
			log.Print("Error converting: " + metricValues[0] + " value: " + metricValues[1] + "to float64")
		}
		payload.Data[0].Metrics = append(payload.Data[0].Metrics, newMetric)
	}

	// Output the payload

	if settings.Instances.Debug {
		btResult, _ := json.MarshalIndent(&payload, "", "  ")
		fmt.Println(string(btResult))
	} else {
		btResult, _ := json.Marshal(&payload)
		fmt.Println(string(btResult))
	}

}
