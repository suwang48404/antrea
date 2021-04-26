/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

const (
	electionID                = "fed-election.antrea.io"
	defaultConfigPath         = "/etc/antrea/fed-manager.conf"
	defaultLeaderElectionFlag = false
	defaultMetricsAddress     = ":8080"
	defaultDebugLogFlag       = false
)

// controllerConfig parses configuration file in yaml.
type controllerConfig struct {
	//TODO remove later
	// SampleStr parses input as string
	SampleStr string `yaml:"sampleStr,omitempty"`

	// SampleInt parses input as string
	SampleInt int `yaml:"sampleInt,omitempty"`

	// SampleBool parses input as string
	SampleBool bool `yaml:"sampleBool,omitempty"`
}

// newConfig populate controller populate and returns controllerConfig.
func newConfig(path string) (*controllerConfig, error) {
	config := controllerConfig{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.UnmarshalStrict(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// validate assigns default value and validate input parameters.
func (c *controllerConfig) validate() error {
	return nil
}
