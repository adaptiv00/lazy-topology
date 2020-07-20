package main

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

func funcMap() template.FuncMap {
	return template.FuncMap{
		"nodes":                    nodes,
		"hosts_with_ports":         hostsWithPorts,
		"join":                     join,
		"with_port":                withPort,
		"with_port_and_prefix":     withPortAndPrefix,
		"http_with_port":           httpWithPort,
		"http_with_port_and_query": httpWithPortAndQuery,
		"https_with_port":          httpsWithPort,
		"with_2ports":              with2Ports,
		"idx":                      idx,
		"get":                      get,
		"grep_key":                 grepKey,
		"grep_1st_value":           grep1stValue,
	}
}

func hostsWithPorts(instances []interface{}, portIndexes ...int) []interface{} {
	res := make([]interface{}, len(instances))
	for idx, instance := range instances {
		node := instance.(map[string]interface{})["node"]
		ports := make([]string, len(portIndexes))
		for portIdx, value := range portIndexes {
			intPort := int(instance.(map[string]interface{})["ports"].([]interface{})[value].(float64))
			ports[portIdx] = fmt.Sprintf("%d", intPort)
		}
		portsString := strings.Join(ports, ":")
		res[idx] = fmt.Sprintf("%s:%s", node, portsString)
	}
	return res
}

func withPort(portIndex int, instances []interface{}) []interface{} {
	return urlWithPort("", "", portIndex, instances)
}

func httpWithPort(portIndex int, instances []interface{}) []interface{} {
	return urlWithPort("http://", "", portIndex, instances)
}

func httpsWithPort(portIndex int, instances []interface{}) []interface{} {
	return urlWithPort("https://", "", portIndex, instances)
}

func httpWithPortAndQuery(portIndex int, queryString string, instances []interface{}) []interface{} {
	return urlWithPort("http://", queryString, portIndex, instances)
}

func withPortAndPrefix(portIndex int, prefix string, instances []interface{}) []interface{} {
	return urlWithPort(prefix, "", portIndex, instances)
}

func urlWithPort(httpPrefix string, querySuffix string, portIndex int, instances []interface{}) []interface{} {
	res := make([]interface{}, len(instances))
	for idx, instance := range instances {
		node := instance.(map[string]interface{})["node"]
		intPort := int(instance.(map[string]interface{})["ports"].([]interface{})[portIndex].(float64))
		res[idx] = fmt.Sprintf("%s%s:%d%s", httpPrefix, node, intPort, querySuffix)
	}
	return res
}

func nodes(instances []interface{}) []interface{} {
	res := make([]interface{}, len(instances))
	for idx, instance := range instances {
		res[idx] = instance.(map[string]interface{})["node"]
	}
	return res
}

func idx(itemIndex int, instances []interface{}) interface{} {
	if itemIndex < len(instances) {
		return instances[itemIndex]
	} else {
		return "index out of bounds"
	}
}

func get(key string, data map[string]interface{}) interface{} {
	return data[key]
}

func with2Ports(port1 int, port2 int, instances []interface{}) []interface{} {
	res := make([]interface{}, len(instances))
	for idx, instance := range instances {
		node := instance.(map[string]interface{})["node"]
		intPort1 := int(instance.(map[string]interface{})["ports"].([]interface{})[port1].(float64))
		intPort2 := int(instance.(map[string]interface{})["ports"].([]interface{})[port2].(float64))
		res[idx] = fmt.Sprintf("%s:%d:%d", node, intPort1, intPort2)
	}
	return res
}

func join(sep string, arr []interface{}) string {
	str := make([]string, len(arr))
	for idx, value := range arr {
		str[idx] = fmt.Sprintf("%v", value)
	}
	return strings.Join(str, sep)
}

func grepKey(containing string, data map[string]interface{}) []interface{} {
	var res []interface{}
	for key, value := range data {
		if strings.Contains(key, containing) {
			res = append(res, value)
		}
	}
	return res
}

func grep1stValue(containing string, data map[string]interface{}) []interface{} {
	var res []interface{}
	for _, value := range data {
		tmp := fmt.Sprintf("%v", value)
		if strings.Contains(tmp, containing) {
			res = append(res, value)
		}
	}
	return res
}

func doRender(tpl template.Template, data map[string]interface{}) (string, error) {
	res := bytes.Buffer{}
	err := tpl.Execute(&res, data)
	if err != nil {
		return "", err
	}
	return res.String(), nil
}
