/*
Copyright 2015 Home Office All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

// showUsage prints the command usage and exits
//
//	message		: an error message to display if exiting with an error
func showUsage(message string, args ...interface{}) {
	flag.PrintDefaults()
	if message != "" {
		fmt.Printf("\n[error] "+message+"\n", args...)
		os.Exit(1)
	}

	os.Exit(0)
}

// hasKey checks to see if a key is present
//
//	key			: the key we are looking for
//	data		: a map of strings to something we are looking at
func hasKey(key string, data map[string]interface{}) bool {
	_, found := data[key]
	return found
}

// getKeys retrieves a list of keys from the map
//
//	data		: the map which you wish to extract the keys from
func getKeys(data map[string]interface{}) []string {
	var list []string
	for key := range data {
		list = append(list, key)
	}
	return list
}

// readConfigFile read in a configuration file
//
//	filename		: the path to the file
func readConfigFile(filename, fileFormat string) (*vaultAuthOptions, error) {
	// step: check the file exists
	if exists, err := fileExists(filename); !exists {
		return nil, fmt.Errorf("the file: %s does not exist", filename)
	} else if err != nil {
		return nil, err
	}
	// step: we only read in json or yaml formats
	suffix := path.Ext(filename)
	switch suffix {
	case ".yaml":
		fallthrough
	case ".yml":
		return readYAMLFile(filename)
	default:
		return readJSONFile(filename, fileFormat)
	}
}

// readJsonFile read in and unmarshall the data into a map
//
//	filename	: the path to the file container the json data
func readJSONFile(filename, format string) (*vaultAuthOptions, error) {
	opts := &vaultAuthOptions{}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	// unmarshall the data
	err = json.Unmarshal(content, &opts)
	if err != nil && format == "default" {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if format == "kubernetes-vault" && opts.ClientToken != "" {
		opts.Method = "token"
		opts.Token = opts.ClientToken
	}

	return opts, nil
}

// readYAMLFile read in and unmarshall the data into a map
//
//	filename	: the path to the file container the yaml data
func readYAMLFile(filename string) (*vaultAuthOptions, error) {
	o := &vaultAuthOptions{}
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(content, o)
	if err != nil {
		return nil, err
	}

	return o, nil
}

// getDurationWithin generate a random integer between min and max
//
//	min			: the smallest number we can accept
//	max			: the largest number we can accept
func getDurationWithin(min, max int) time.Duration {
	jitter := max - min
	if jitter <= 0 {
		jitter = 1
	}
	duration := rand.Intn(jitter) + min
	return time.Duration(duration) * time.Second
}

// getEnv checks to see if an environment variable exists otherwise uses the default
//
//	env			: the name of the environment variable you are checking for
//	value		: the default value to return if the value is not there
func getEnv(env, value string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}

	return value
}

// fileExists checks to see if a file exists
//
//	filename		: the full path to the file you are checking for
func fileExists(filename string) (bool, error) {
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// processResource is responsible for generating the specific content from the resource
//
//	rn		: a point to the vault resource
//	data		: a map of the related secret associated to the resource
func processResource(rn *VaultResource, data map[string]interface{}) (err error) {
	// step: determine the resource path
	filename := rn.GetFilename()
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(options.outputDir, filename)
	}
	// step: format and write the file
	switch rn.format {
	case "yaml":
		fallthrough
	case "yml":
		err = writeYAMLFile(filename, data, rn.fileMode)
	case "json":
		err = writeJSONFile(filename, data, rn.fileMode)
	case "ini":
		err = writeIniFile(filename, data, rn.fileMode)
	case "csv":
		err = writeCSVFile(filename, data, rn.fileMode)
	case "env":
		err = writeEnvFile(filename, data, rn.fileMode)
	case "cert":
		err = writeCertificateFile(filename, data, rn.fileMode)
	case "txt":
		err = writeTxtFile(filename, data, rn.fileMode)
	case "flatten":
		err = writeFlattenFiles(filename, data, rn.fileMode)
	case "bundle":
		err = writeCertificateBundleFile(filename, data, rn.fileMode)
	case "credential":
		err = writeCredentialFile(filename, data, rn.fileMode)
	case "template":
		err = writeTemplateFile(filename, data, rn.fileMode, rn.templateFile)
	case "aws":
		err = writeAwsCredentialFile(filename, data, rn.fileMode)
	default:
		return fmt.Errorf("unknown output format: %s", rn.format)
	}
	// step: check for an error
	if err != nil {
		return err
	}

	// step: check if we need to execute a command
	if rn.execPath != "" {
		glog.V(10).Infof("executing the command: %s for resource: %s", rn.execPath, filename)
		parts := strings.Split(rn.execPath, " ")
		var args []string
		if len(parts) > 1 {
			args = parts[1:]
		} else {
			args = []string{filename}
		}

		cmd := exec.Command(parts[0], args...)
		cmd.Start()
		timer := time.AfterFunc(options.execTimeout, func() {
			if err = cmd.Process.Kill(); err != nil {
				glog.Errorf("failed to kill the command, pid: %d, error: %s", cmd.Process.Pid, err)
			}
		})
		// step: wait for the command to finish
		err = cmd.Wait()
		timer.Stop()
	}

	return err
}

// create any necessary parent directories for the file
// (thanks to https://stackoverflow.com/a/54001062/12029637)
func ensureDir(fileName string) error {
	dirName := filepath.Dir(fileName)
	if _, serr := os.Stat(dirName); serr != nil {
		merr := os.MkdirAll(dirName, os.ModePerm)
		if merr != nil {
			return merr
		}
	}
	return nil
}

// writeFile writes the file to stdout or an actual file
func writeFile(filename string, content []byte, mode os.FileMode) error {
	if options.dryRun {
		glog.Infof("dry-run: filename: %s, content:", filename)
		fmt.Printf("%s\n", string(content))
		return nil
	}
	glog.V(3).Infof("saving the file: %s", filename)

	err := ensureDir(filename)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, content, mode)
}
