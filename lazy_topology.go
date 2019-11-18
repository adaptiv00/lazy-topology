package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const TopologyRootFolder = "."
const TopologyDefFile = "topology.txt"
const DefaultServicesFolder = "services"
const DefaultDeployFolder = "deploy"
const DefaultBinFolder = "bin"
const DefaultSwarmStackName = "app"
const SwarmServiceFragment = "swarm-service"
const DefaultSwarmDeployFolder = "swarm"
const TemplateExt = ".tmpl"
const DefaultFileMode = os.FileMode(0644)

func main() {
	var topology, err = BuildTopologyFromFile(topologyFile())
	handleError(err)
	err = renderAllFor(*topology)
	handleError(err)
}

type FileScanningContext struct {
	rootPath              string
	pathFragment          string
	includingPathFragment bool
	extension             string
}

type RenderTemplate func(templateFilePath string) (string, error)

func renderAllFor(topology Topology) error {

	var err = os.RemoveAll(deployDir())
	if err != nil {
		return err
	}

	err = appendToFile(path.Join(deployDir(), "topology.json"), topology.jsonString)
	if err != nil {
		return err
	}

	err = renderSwarmServiceTemplates(topology)
	if err != nil {
		return err
	}

	err = renderAllButSwarmServiceTemplates(topology)
	if err != nil {
		return err
	}

	err = renderGlobalTemplates(topology)
	if err != nil {
		return err
	}

	err = appendServicePrePostDeploysToGlobal(topology)
	if err != nil {
		return err
	}

	return nil
}

func renderSwarmServiceTemplates(topology Topology) error {
	stackMap := map[string]string{}
	for _, serviceDef := range topology.serviceMetadata {
		stackMap[serviceDef.Config.getString("stack", DefaultSwarmStackName)] = ""
	}

	for _, serviceDef := range topology.serviceMetadata {

		fileScanningContext := FileScanningContext{
			rootPath:              serviceDir(serviceDef.Name),
			pathFragment:          SwarmServiceFragment,
			includingPathFragment: true,
			extension:             TemplateExt,
		}

		var renderSwarmServiceTemplate = func(templateFilePath string) (string, error) {
			results, err := RenderServiceTemplate(templateFilePath, serviceDef.Name, topology.dataMap)
			return strings.Join(results, "\n"), err
		}

		services, err := withScanningContext(fileScanningContext, renderSwarmServiceTemplate)
		if err != nil {
			return err
		}

		stackMap[serviceDef.Config.getString("stack", DefaultSwarmStackName)] += strings.Join(services, "")

	}

	for stackName, stackContent := range stackMap {
		if stackContent == "" {
			continue
		}
		swarmData := map[string]interface{}{
			"content": stackContent,
		}
		var swarmStackString, err = RenderTemplateString(swarmWrapper, swarmData)
		if err != nil {
			return err
		}
		stackFilePath := path.Join(deployDir(), DefaultSwarmDeployFolder, fmt.Sprintf("%s.yml", stackName))
		err = appendToFile(stackFilePath, swarmStackString)
		if err != nil {
			return err
		}
	}
	return nil
}

func renderAllButSwarmServiceTemplates(topology Topology) error {
	for _, serviceDef := range topology.serviceMetadata {

		fileScanningContext2 := FileScanningContext{
			rootPath:              serviceDir(serviceDef.Name),
			pathFragment:          SwarmServiceFragment,
			includingPathFragment: false,
			extension:             TemplateExt,
		}

		var _renderGenericTemplate = func(templateFilePath string) (string, error) {
			return "", renderGenericTemplate(templateFilePath, serviceDef, topology)
		}

		_, err2 := withScanningContext(fileScanningContext2, _renderGenericTemplate)
		if err2 != nil {
			return err2
		}

	}
	return nil
}

// Global templates end up full relative path in deploy folder
// aka ./bin/__utils.sh.tmpl ends up in deploy/bin/__utils.sh
func renderGlobalTemplates(topology Topology) error {

	var renderGlobalTemplate = func(filePath string) (string, error) {
		outFilePath := strings.ReplaceAll(path.Join(DefaultDeployFolder, filePath), TemplateExt, "")
		var res, err = RenderGlobalTemplate(filePath, topology)
		if err != nil {
			return "", err
		}
		return "", appendToFile(outFilePath, res)
	}

	_, err := withDirectory(DefaultBinFolder, renderGlobalTemplate)
	return err
}

func appendServicePrePostDeploysToGlobal(topology Topology) error {
	err := appendDeployToGlobal("post-deploy.sh", topology)
	if err != nil {
		return err
	}
	return appendDeployToGlobal("pre-deploy.sh", topology)
}

func appendDeployToGlobal(fileName string, topology Topology) error {
	deployFile := path.Join(DefaultDeployFolder, DefaultBinFolder, fileName)
	if _, err := os.Stat(deployFile); err == nil  {
		for _, serviceDef := range topology.serviceMetadata {
			servicePostDeployFile := path.Join(DefaultDeployFolder, serviceDef.Name, DefaultBinFolder, fileName)
			if _, err := os.Stat(servicePostDeployFile); err == nil {
				content, err := readTextFile(servicePostDeployFile)
				if err != nil {
					return err
				}
				content = strings.ReplaceAll(content,"#!/usr/bin/env bash\n", "")
				err = appendToFile(deployFile, content)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func renderGenericTemplate(templateFilePath string, serviceDef ServiceMetadata, topology Topology) error {
	results, err := RenderServiceTemplate(templateFilePath, serviceDef.Name, topology.dataMap)
	if err != nil {
		return err
	}
	for idx, res := range results {
		tmp := strings.ReplaceAll(templateFilePath, ServicesFolder, DefaultDeployFolder)
		tmp1 := strings.ReplaceAll(tmp, TemplateExt, "")
		resultFilePath := strings.ReplaceAll(tmp1, "~", fmt.Sprintf("-%s", nodeId(idx)))
		err = appendToFile(resultFilePath, res)
		if err != nil {
			return err
		}
	}
	return nil
}

func readTextFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	b, err := ioutil.ReadAll(file)
	return string(b), err
}

func appendToFile(filePath string, content string) error {
	// Make the dirs up to the file
	err := MkDirs(path.Dir(filePath))
	if err != nil {
		return err
	}
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, DefaultFileMode)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(content)); err != nil {
		err1 := f.Close() // ignore error; Write error takes precedence
		if err1 != nil {
			return err1
		}
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func topologyFile() string {
	return path.Join(TopologyRootFolder, TopologyDefFile)
}

func serviceDir(serviceName string) string {
	return path.Join(servicesDir(), serviceName)
}

func servicesDir() string {
	return path.Join(TopologyRootFolder, DefaultServicesFolder)
}

func deployDir() string {
	return path.Join(TopologyRootFolder, DefaultDeployFolder)
}

func withScanningContext(ctx FileScanningContext, render RenderTemplate) ([]string, error) {
	var res []string
	err := filepath.Walk(ctx.rootPath, func(templateFilePath string, handle os.FileInfo, err error) error {
		nameMatches := ctx.includingPathFragment && strings.Contains(templateFilePath, ctx.pathFragment) ||
			!ctx.includingPathFragment && !strings.Contains(templateFilePath, ctx.pathFragment)
		if !handle.IsDir() && nameMatches && path.Ext(handle.Name()) == ctx.extension {
			tmp, err := render(templateFilePath)
			if err != nil {
				return err
			}
			res = append(res, tmp)
		}
		return nil
	})
	return res, err
}

func withDirectory(rootPath string, render RenderTemplate) ([]string, error) {
	var res []string
	err := filepath.Walk(rootPath, func(templateFilePath string, handle os.FileInfo, err error) error {
		if !handle.IsDir() && path.Ext(handle.Name()) == TemplateExt {
			tmp, err := render(templateFilePath)
			if err != nil {
				return err
			}
			res = append(res, tmp)
		}
		return nil
	})
	return res, err
}

func MkDirs(path string) error {
	cmd := exec.Command("mkdir", "-p", path)
	err := cmd.Run()
	return err
}

func handleError(err error) {
	if err != nil {
		panic(err)
	}
}

const swarmWrapper = `# No good reason this is 3.7
version: "3.7"

services:
    {{ .content }}
networks:
  host_net:
    external: true
    name: host

`
