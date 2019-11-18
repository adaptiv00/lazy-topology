package main

import (
	"encoding/json"
	"testing"
)

func TestBuildTopology(t *testing.T) {
	topology, err := BuildTopologyFromFile("topology.txt")
	if err != nil {
		t.Errorf("Unable to read topology, err: %s", err)
	}
	content := topology.dataMap
	mustContainKey("node_count", content, t)
	//mustContainKey("node_names", content, t)
	//mustContainKey("service_names", content, t)
	mustContainKey("zookeeper", content, t)

	zookeeperService, _ := content["zookeeper"].(map[string]interface{})
	//mustContainKey("name", zookeeperService, t)
	mustContainKey("instances", zookeeperService, t)
	zookeeper1stInstance := instance(zookeeperService, 0)
	MustBeInt(2181, port(zookeeper1stInstance, 0), "first instance first port", t)
	zookeeper2ndInstance := instance(zookeeperService, 1)
	MustBeInt(2182, port(zookeeper2ndInstance, 0), "second instance first port", t)
}

func mustContainKey(key string, content map[string]interface{}, t *testing.T) {
	if content[key] == nil {
		nodeJson, _ := json.Marshal(content)
		t.Errorf("Should have key '%s',\njson: %s", key, string(nodeJson))
	}
}

func instance(content map[string]interface{}, index int) interface{} {
	return content["instances"].([]interface{})[index]
}

func port(node interface{}, index int) int {
	ports := node.(map[string]interface{})["ports"].([]interface{})
	return int(ports[0].(float64))
}
