package main

import (
	"path"
	"strings"
	"text/template"
)

func RenderServiceTemplate(fileName string, serviceName string, topology map[string]interface{}) ([]string, error) {
	data := make(map[string]interface{})
	data["topology"] = topology
	data["service"] = topology[serviceName]
	if strings.Contains(fileName, "~") {
		var res []string
		for _, instance := range topology[serviceName].(map[string]interface{})["instances"].([]interface{}) {
			data["instance"] = instance
			tmp, err := RenderTemplateFile(fileName, data)
			if err != nil {
				return nil, err
			}
			res = append(res, tmp)
		}
		return res, nil
	} else {
		content, err := RenderTemplateFile(fileName, data)
		return []string { content, }, err
	}
}

func RenderGlobalTemplate(fileName string, topology Topology) (string, error) {
	var services []interface{}
	for _, serviceDef := range topology.serviceMetadata {
		services = append(services, topology.dataMap[serviceDef.Name])
	}
	data := map[string]interface{}{}
	tmp := topology.dataMap
	// Ability to range over a service collection, they're originally root nodes
	tmp["services"] = services
	data["topology"] = tmp

	return RenderTemplateFile(fileName, data)
}

func RenderTemplateFile(fileName string, data map[string]interface{}) (string, error) {
	tpl := template.Must(template.New(path.Base(fileName)).Funcs(funcMap()).ParseFiles(fileName))
	return doRender(*tpl, data)
}

func RenderTemplateString(templateContent string, data map[string]interface{}) (string, error) {
	tpl := template.Must(template.New("").Funcs(funcMap()).Parse(templateContent))
	return doRender(*tpl, data)
}
