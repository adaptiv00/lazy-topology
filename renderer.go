package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"text/template"
)

func RenderServiceTemplate(fileName string, serviceName string, topology map[string]interface{}) ([]string, error) {
	data := make(map[string]interface{})
	data["topology"] = topology
	data["service"] = topology[serviceName]
	serviceConfigMap := data["service"].(map[string]interface{})["config"].(map[string]interface{})
	if strings.Contains(fileName, "~") {
		var res []string
		for _, instance := range topology[serviceName].(map[string]interface{})["instances"].([]interface{}) {
			data["instance"] = instance
			tmp, err := RenderTemplateFile(fileName, data)
			if err != nil {
				return nil, err
			}
			res = append(res, replacePlaceholders(tmp, serviceConfigMap))
		}
		return res, nil
	} else {
		content, err := RenderTemplateFile(fileName, data)
		return []string { replacePlaceholders(content, serviceConfigMap) }, err
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

// Kind of hacky way to inject env vars, maybe yml parsing and appending would work better?
// So this works in two steps:
// 1.) Add MY_PREFIX_LAZY_PLACEHOLDER anywhere in service yml
// 2.) Add MY_PREFIX_VAR1=the_value in service config
// Result is: MY_PREFIX_LAZY_PLACEHOLDER entire line gets replaced with VAR1: the_value
// Useful for defining environment vars in config
func replacePlaceholders(content string, config map[string]interface{}) string {
	lineMatcher := regexp.MustCompile(".*LAZY_PLACEHOLDER.*")
	for _, lineMatch := range lineMatcher.FindAll([]byte(content), -1) {
		prefix := strings.TrimSpace(strings.ReplaceAll(strings.Split(string(lineMatch), ":")[0], "LAZY_PLACEHOLDER", ""))
		indentMatch := regexp.MustCompile("(\\s*)\\w*")
		spacePrefixMatch := indentMatch.FindStringSubmatch(string(lineMatch))
		spacePrefix := ""
		if len(spacePrefixMatch) >= 2 {
			spacePrefix = spacePrefixMatch[1]
		}
		var varMatches []string
		for varMatch := range config {
			if strings.HasPrefix(varMatch, prefix) {
				//println(config[varMatch].(string))
				varName := strings.ReplaceAll(varMatch, prefix, "")
				varValue := config[varMatch].(string)
				varMatches = append(varMatches, fmt.Sprintf("%s%s: %s", spacePrefix, varName, varValue))
			}
		}
		if len(varMatches) > 0 {
			content = lineMatcher.ReplaceAllString(content, strings.Join(varMatches, "\n"))
		}
	}
	return content
}