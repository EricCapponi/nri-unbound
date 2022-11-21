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

const settingsFile = "/var/db/newrelic-infra/custom-integrations/unboundCollector.yml"

var unboundControl = "unbound-control"

type settings struct {
	Unbound struct {
		Cfgfile string
		Server  string
	}
	Collector struct {
		Name        string `yaml:"name"`
		ControlPath string `yaml:"controlPath"`
		EntityName  string `yaml:"entityName"`
		EntityType  string `yaml:"entityType"`
		DisplayName string `yaml:"displayName"`
		Debug       bool   `yaml:"debug,omitempty"`
		Reset       bool   `yaml:"reset,omitempty"`
		// TODO add metatdata, an array of key : value pairs
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
		Name        string   `json:"name"`
		Type        string   `json:"type"`
		DisplayName string   `json:"displayName"`
		Metadata    struct{} `json:"metadata"`
	}
	Metrics []metricsType
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
	buf, err := ioutil.ReadFile(settingsFile)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", settingsFile, err)
	}

	c := &settings{}
	err = yaml.Unmarshal(buf, c)
	//err = yaml.v3.Unmarshal(buf, c)
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
	payload.Integration.Name = settings.Collector.Name
	payload.Integration.Version = "0.1"

	var entity entityType
	entity.Entity.Name = settings.Collector.EntityName
	entity.Entity.Type = settings.Collector.EntityType
	entity.Entity.DisplayName = settings.Collector.DisplayName

	payload.Data = append(payload.Data, entity)

	// build the parameter list for the command
	var args string

	if settings.Unbound.Cfgfile != "" {
		args = "-c " + settings.Unbound.Cfgfile
	}
	if settings.Unbound.Server != "" {
		if args != "" {
			args += " "
		}
		args += "-s " + settings.Unbound.Server
	}

	// Run the command to collect metircs
	cmdStruct := exec.Command(settings.Collector.ControlPath+unboundControl, args)
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

	// // run the command

	// Output the payload

	if settings.Collector.Debug {
		btResult, _ := json.MarshalIndent(&payload, "", "  ")
		fmt.Println(string(btResult))
	} else {
		btResult, _ := json.Marshal(&payload)
		fmt.Println(string(btResult))
	}

}
