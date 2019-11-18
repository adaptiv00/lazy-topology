package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

const KeyValueSeparator = "="
const ServiceConfigSuffix = "_cfg"
const InstancePortSeparator = ":"
const ValueSeparator = ","
const NodeNamePrefix = "dev-"

type InstanceDef struct {
	ID    string
	Index int    `json:"index"`
	Node  string `json:"node"`
	Name  string `json:"name"`
	Ports []int  `json:"ports"`
}

type ServiceDef struct {
	Name      string            `json:"name"`
	Instances []InstanceDef     `json:"instances"`
	Config    map[string]string `json:"config"`
}

type Topology struct {
	topologyMetadata *TopologyMetadata
	serviceMetadata  []ServiceMetadata
	dataMap          map[string]interface{}
	jsonString       string
}

func BuildTopologyFromFile(fileName string) (*Topology, error) {
	topologyString, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return BuildTopologyFromString(string(topologyString))
}

func BuildTopologyFromString(topologyString string) (*Topology, error) {
	return BuildTopologyFromLines(strings.Split(topologyString, "\n"))
}

func BuildTopologyFromLines(lines []string) (*Topology, error) {
	res := map[string]interface{}{}
	topologyMetadata, err := TopologyMetadataFromLines(lines)
	if err != nil {
		return nil, err
	}
	serviceMetadataList, err := ServiceMetadataFromLines(lines, *topologyMetadata)
	if err != nil {
		return nil, err
	}
	serviceDefs := make([]ServiceDef, len(serviceMetadataList))
	portsCache := make(map[string]string)
	for idx, serviceMetadata := range serviceMetadataList {
		serviceDef, err := serviceDefFromMetadata(serviceMetadata, *topologyMetadata, portsCache)
		if err != nil {
			return nil, err
		}
		res[serviceDef.Name] = *serviceDef
		serviceDefs[idx] = *serviceDef
	}

	res["node_count"] = topologyMetadata.NodeCount
	res["config"] = topologyMetadata.Config.data
	res["node_names"] = getNodeNames(topologyMetadata.NodeCount)
	res["stack_names"] = getStackNames(topologyMetadata.Config.getString("stack", DefaultSwarmStackName), serviceMetadataList)

	jsonString, err := TopologyToJSonString(res)
	if err != nil {
		return nil, err
	}
	dataMap := map[string]interface{}{}
	err1 := json.Unmarshal([]byte(jsonString), &dataMap)

	return &Topology{
		topologyMetadata: topologyMetadata,
		serviceMetadata:  serviceMetadataList,
		dataMap:          dataMap,
		jsonString:       jsonString,
	}, err1
}

func TopologyToJSonString(topology map[string]interface{}) (string, error) {
	topologyJson, err := json.MarshalIndent(topology, "", "  ")
	if err != nil {
		return "", err
	}
	return string(topologyJson) + "\n", nil
}

func serviceDefFromMetadata(service ServiceMetadata, topology TopologyMetadata, portsCache map[string]string) (*ServiceDef, error) {
	instanceDefs := make([]InstanceDef, len(service.NodeIDs))
	nodeNames := getNodeNames(topology.NodeCount)
	for idx, nodeID := range service.NodeIDs {
		id := nodeId(idx)
		instanceDefs[idx] = InstanceDef{
			ID:    id,
			Index: idx,
			Node:  nodeNames[nodeID-1],
			Name:  fmt.Sprintf("%s-%s", service.Name, id),
			Ports: findAndRegisterPorts(service.Ports, nodeNames[nodeID-1], portsCache),
		}
	}
	return &ServiceDef{
		Name:      service.Name,
		Instances: instanceDefs,
		Config:    service.Config.data,
	}, nil
}

func findAndRegisterPorts(basePorts []int, node string, portsCache map[string]string) []int {
	ports := make([]int, len(basePorts))
	for idx, basePort := range basePorts {
		ports[idx] = findAndRegisterPort(basePort, node, portsCache)
	}
	return ports
}

func findAndRegisterPort(basePort int, node string, portsCache map[string]string) int {
	port := basePort
	for portsCache[fmt.Sprintf("%s:%d", node, port)] != "" {
		port += 1
	}
	portsCache[fmt.Sprintf("%s:%d", node, port)] = fmt.Sprintf("%s:%d", node, port)
	return port
}

func nodeName(idx int) string {
	return fmt.Sprintf("%snode%02d", NodeNamePrefix, idx+1)
}

func nodeId(idx int) string {
	return fmt.Sprintf("%02d", idx+1)
}

func getNodeNames(nodeCount int) []string {
	nodes := make([]string, nodeCount)
	for idx := 0; idx < nodeCount; idx++ {
		nodes[idx] = nodeName(idx)
	}
	return nodes
}

func getStackNames(defaultStack string, serviceDefs []ServiceMetadata) []string {
	cache := map[string]interface{}{}
	var names []string
	for _, serviceDef := range serviceDefs {
		stackName := serviceDef.Config.getString("stack", defaultStack)
		if cache[stackName] == nil {
			names = append(names, stackName)
			cache[stackName] = ""
		}
	}
	return names
}