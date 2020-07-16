package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	data map[string]string
}

func (config Config) getString(name string, dfolt string) string {
	var tmp, exists = config.data[name]
	if exists {
		return tmp
	}
	return dfolt
}

func (config Config) dataForRender() map[string]interface{} {
	res := map[string]interface{}{}
	for key, v := range config.data {
		res[key] = v
	}
	topologyData := map[string]interface{}{}
	topologyConfigData := map[string]interface{}{}
	topologyConfigData["config"] = res
	topologyData["topology"] = topologyConfigData
	return topologyData
}

type TopologyMetadata struct {
	NodeCount int
	NodeNames []string
	Config    Config
}

type ServiceMetadata struct {
	Name      string // zookeeper
	NodeIDs   []int  // 1,2,3 -> [1, 2, 3]
	Ports     []int  // 2181,2888,3888 -> [2181, 2888, 3888]
	Config    Config
	RawConfig Config
}

func ReadKeyValuePair(line string) (string, string, error) {
	if ! strings.Contains(line, KeyValueSeparator) {
		return "", "", errors.New("incorrect line format. Use: key=value")
	}
	keyAndValue := strings.Split(strings.TrimSpace(line), KeyValueSeparator)
	key := strings.TrimSpace(keyAndValue[0])
	// if it split in more than 2, it means value contains at least one KeyValueSeparator and we need to join by it
	value := strings.TrimSpace(strings.Join(keyAndValue[1:], KeyValueSeparator))
	return key, value, nil
}

func TopologyMetadataFromLines(lines []string) (*TopologyMetadata, error) {
	res := map[string]interface{}{}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if shouldIgnore(line) { // # comment, ignore empty lines
			continue
		}
		key, value, err := ReadKeyValuePair(line)
		if err != nil {
			return nil, err
		}
		if strings.Contains(key, ServiceConfigSuffix) { // ignore service defs for now
			continue
		}
		res[key] = value
	}
	// Validate node_count
	if res["node_count"] == nil {
		return nil, errors.New("topology needs to contain a 'node_count' field")
	}
	// Must be integer
	nodeCount, err := strconv.Atoi(res["node_count"].(string))
	if err != nil {
		return nil, err
	}

	inheritConfigFile := inheritTopologyConfigFile()
	inheritConfig, err := ReadConfigFile(inheritConfigFile, nil, nil)
	if err != nil {
		log.Println(fmt.Sprintf("ingnoring missing inherit topology config: '%s'", inheritConfigFile))
	} else {
		log.Println(fmt.Sprintf("inheriting topology config: '%s'", inheritConfigFile))
	}
	topologyConfig, err := ReadConfigFile(topologyConfigFile(), nil, &inheritConfig)
	if err != nil {
		return nil, err
	}

	return &TopologyMetadata{
		NodeCount: nodeCount,
		NodeNames: getNodeNames(topologyConfig.getString(NodeNamePrefixPropertyName, DefaultNodeNamePrefix), nodeCount),
		Config:    topologyConfig,
	}, nil
}

func NewConfig(data map[string]string, parent *Config) Config {
	res := map[string]string{}
	// initialize with parent if there
	if parent != nil {
		for k, v := range parent.data {
			res[k] = v
		}
	}
	// override parent
	for k, v := range data {
		res[k] = v
	}
	return Config{
		data: res,
	}
}

func EmptyConfig() Config {
	return NewConfig(map[string]string{}, nil)
}

// config files support templating if you pass in a non nil templateData
func ReadConfigFile(configFile string, templateData map[string]interface{}, parent *Config) (Config, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return EmptyConfig(), nil
	}
	configFileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return EmptyConfig(), err
	}
	return ReadConfigString(string(configFileBytes), templateData, parent)
}

func ReadConfigString(configFileContents string, templateData map[string]interface{}, parent *Config) (Config, error) {
	res := map[string]string{}
	var renderedConfigFileContents = configFileContents
	var err error
	if templateData != nil {
		renderedConfigFileContents, err = RenderTemplateString(configFileContents, templateData)
		if err != nil {
			return EmptyConfig(), err
		}
	}
	for _, line := range strings.Split(renderedConfigFileContents, "\n") {
		if shouldIgnore(line) {
			continue
		}
		key, value, err := ReadKeyValuePair(line)
		if err != nil {
			return EmptyConfig(), err
		}
		res[key] = value
	}
	return NewConfig(res, parent), nil
}

func parseServiceMetadata(name, spec string, topologyMetadata TopologyMetadata) (*ServiceMetadata, error) {
	nodeIDsAndPorts := strings.Split(strings.TrimSpace(spec), InstancePortSeparator)
	nodeIDsAsStrings := strings.Split(strings.TrimSpace(nodeIDsAndPorts[0]), ValueSeparator)
	var nodeIDs []int
	for _, nodeID := range nodeIDsAsStrings {
		nodeId, err := strconv.Atoi(strings.TrimSpace(nodeID))
		if err != nil {
			return nil, err
		}
		if nodeId < 1 || nodeId > topologyMetadata.NodeCount {
			return nil, errors.New(fmt.Sprintf("node id needs to be between 1 and %d, inclusive",
				topologyMetadata.NodeCount))
		}
		nodeIDs = append(nodeIDs, nodeId)
	}
	var ports []int
	portsAsStrings := strings.Split(strings.TrimSpace(strings.Join(nodeIDsAndPorts[1:], "")), ValueSeparator)
	for _, portString := range portsAsStrings {
		port, err := strconv.Atoi(strings.TrimSpace(portString))
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	topologyConfigData := topologyMetadata.Config.dataForRender()
	inheritConfigFilePath := inheritServiceConfigFile(name)
	inheritServiceConfig, err := ReadConfigFile(inheritConfigFilePath, topologyConfigData, &topologyMetadata.Config)
	if _, err1 := os.Stat(inheritConfigFilePath); err != nil || os.IsNotExist(err1) {
		//log.Println(fmt.Sprintf("ignoring missing inherit service config: '%s'", inheritConfigFilePath))
	} else {
		log.Println(fmt.Sprintf("inheriting '%s' from config: '%s'", name, inheritConfigFilePath))
	}

	configFilePath := serviceConfigFilePath(name)
	serviceConfig, err := ReadConfigFile(configFilePath, topologyConfigData, &inheritServiceConfig)
	if err != nil {
		return nil, err
	}
	// service config w/o topology data, strictly for JSON rendering. DO NOT USE for rendering
	inheritRawConfig, err := ReadConfigFile(inheritConfigFilePath, topologyConfigData, nil)
	if err != nil {
		return nil, err
	}
	rawConfig, err := ReadConfigFile(configFilePath, topologyConfigData, &inheritRawConfig)
	if err != nil {
		return nil, err
	}

	return &ServiceMetadata{
		Name:      name,
		NodeIDs:   nodeIDs,
		Ports:     ports,
		Config:    serviceConfig,
		RawConfig: rawConfig,
	}, nil
}

func ServiceMetadataFromLines(lines []string, topologyMetadata TopologyMetadata) ([]ServiceMetadata, error) {
	var metas []ServiceMetadata
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if shouldIgnore(line) { // # comment, ignore empty lines
			continue
		}
		key, serviceSpec, err := ReadKeyValuePair(line)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(key, ServiceConfigSuffix) { // ignore all but service defs
			continue
		}
		if !strings.Contains(serviceSpec, InstancePortSeparator) {
			return nil, errors.New(fmt.Sprintf("'%s' must have '%s'", key, InstancePortSeparator))
		}
		serviceName := strings.ReplaceAll(key, ServiceConfigSuffix, "")
		meta, err := parseServiceMetadata(serviceName, serviceSpec, topologyMetadata)
		if err != nil {
			return nil, err
		}
		metas = append(metas, *meta)
	}
	return metas, nil
}

func shouldIgnore(line string) bool {
	return strings.Index(line, "#") == 0 || strings.TrimSpace(line) == ""
}
