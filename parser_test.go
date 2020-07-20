package main

import (
	"strings"
	"testing"
)

func TestTopologyMetadataFromLines(t *testing.T) {
	topologyMetadata, err := TopologyMetadataFromLines(strings.Split(TopologyString, "\n"))
	handleTestingError(err, t)
	MustBeInt(2, topologyMetadata.NodeCount, "node_count", t)
}

func TestServiceMetadataFromLines(t *testing.T) {
	topologyMetadata, err := TopologyMetadataFromLines(strings.Split(TopologyString, "\n"))
	handleTestingError(err, t)
	serviceMetadataList, err := ServiceMetadataFromLines(strings.Split(TopologyString, "\n"), *topologyMetadata, true)
	MustBeInt(1, len(serviceMetadataList), "service def count", t)
	serviceMetadata := serviceMetadataList[0]
	MustBeInt(3, len(serviceMetadata.NodeIDs), "node id count", t)
	MustBeInt(3, len(serviceMetadata.Ports), "port count", t)
	MustBeInt(1, serviceMetadata.NodeIDs[0], "1st node id", t)
	MustBeInt(1, serviceMetadata.NodeIDs[1], "2nd node id", t)
	MustBeInt(2, serviceMetadata.NodeIDs[2], "3rd node id", t)
	MustBeInt(2181, serviceMetadata.Ports[0], "1st port", t)
	MustBeInt(2888, serviceMetadata.Ports[1], "2nd port", t)
	MustBeInt(3888, serviceMetadata.Ports[2], "3rd port", t)
}

func TestReadConfigContent(t *testing.T) {
	topologyConfig, err := ReadConfigString(TopologyConfig, map[string]interface{}{}, nil)
	handleTestingError(err, t)
	serviceConfig, err := ReadConfigString(ServiceConfig, topologyConfig.dataForRender(), &topologyConfig)
	MustBeString("/home/app/zookeeper", serviceConfig.getString("runtime_folder", ""), "runtime_folder config", t)
}

func MustBeInt(expected, current int, what string, t *testing.T) {
	if current != expected {
		t.Errorf("%s is '%d' but should be '%d'", what, current, expected)
	}
}

func MustBeString(expected, current string, what string, t *testing.T) {
	if current != expected {
		t.Errorf("%s is '%s' but should be '%s'", what, current, expected)
	}
}

func handleTestingError(err error, t *testing.T) {
	if err != nil {
		t.Errorf("unable to read topology metadata, err: %s", err)
	}
}

const TopologyString = `# Ignore comments, extra space and empty lines

node_count     = 2 
zookeeper_cfg  = 1,1, 2: 2181, 2888,3888  
`
const TopologyConfig = `# Ignore comments, extra space and empty lines

stack          = app 
runtime_folder = /home/app
app_user       = app
`

const ServiceConfig = `# Ignore comments, extra space and empty lines
runtime_folder = {{ .topology.config.runtime_folder }}/zookeeper
stack          = kafka
`