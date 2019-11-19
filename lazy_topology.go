package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const DefaultSwarmStackName = "app"
const SwarmServiceFragment = "swarm-service"
const DefaultSwarmDeployFolder = "swarm"

func main() {
	var topology, err = BuildTopologyFromFile(topologyFile())
	handleError(err)
	err = renderAllFor(*topology)
	handleError(err)
}

func renderAllFor(topology Topology) error {

	var err = os.RemoveAll(deployDir())
	if err != nil {
		return err
	}

	err = appendToFile(topologyJsonFile(), topology.jsonString)
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

		inheritFileScanningContext := FileScanningContext{
			rootPath:              inheritServiceDir(serviceDef.Name),
			pathFragment:          SwarmServiceFragment,
			includingPathFragment: true,
			extension:             TemplateExt,
		}

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

		inheritServices, err := withScanningContext(inheritFileScanningContext, renderSwarmServiceTemplate)
		if err != nil {
			return err
		}

		// TODO REALLY ADD EXCLUDES HERE AS IT'S NOT OVERWRITING, IT'S ADDING
		services, err := withScanningContext(fileScanningContext, renderSwarmServiceTemplate)
		if err != nil {
			return err
		}

		servicesString := strings.Join(append(inheritServices, services...), "")
		stackMap[serviceDef.Config.getString("stack", DefaultSwarmStackName)] += servicesString

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
		err = appendToFile(stackFilePath(stackName), swarmStackString)
		if err != nil {
			return err
		}
	}
	return nil
}

func renderAllButSwarmServiceTemplates(topology Topology) error {
	for _, serviceDef := range topology.serviceMetadata {

		inheritFileScanningContext := FileScanningContext{
			rootPath:              inheritServiceDir(serviceDef.Name),
			pathFragment:          SwarmServiceFragment,
			includingPathFragment: false,
			extension:             TemplateExt,
		}

		fileScanningContext2 := FileScanningContext{
			rootPath:              serviceDir(serviceDef.Name),
			pathFragment:          SwarmServiceFragment,
			includingPathFragment: false,
			extension:             TemplateExt,
		}

		var _renderGenericTemplate = func(templateFilePath string) (string, error) {
			return "", renderGenericTemplate(templateFilePath, serviceDef, topology)
		}

		_, err := withScanningContext(inheritFileScanningContext, _renderGenericTemplate)
		if err != nil {
			return err
		}
		_, err = withScanningContext(fileScanningContext2, _renderGenericTemplate)
		if err != nil {
			return err
		}

	}
	return nil
}

// Global templates end up full relative path in deploy folder
// aka ./bin/__utils.sh.tmpl ends up in deploy/bin/__utils.sh
func renderGlobalTemplates(topology Topology) error {

	inheritRootFolder := fmt.Sprintf("%s/", inheritRootDir())
	var renderGlobalTemplate = func(filePath string) (string, error) {
		// remove .tmpl and inherit root folder path in vendors, remains only what's in it
		outFilePath := strings.ReplaceAll(deployFilePath(filePath), TemplateExt, "")
		outFilePath = strings.ReplaceAll(outFilePath, inheritRootFolder, "")
		var res, err = RenderGlobalTemplate(filePath, topology)
		if err != nil {
			return "", err
		}
		return "", appendToFile(outFilePath, res)
	}
	// Non existing paths will be ignored
	_, err := withDirectory(rootInheritBinFolder(), renderGlobalTemplate)
	if err != nil {
		return err
	}
	_, err = withDirectory(BinFolder, renderGlobalTemplate)
	return err
}

func appendServicePrePostDeploysToGlobal(topology Topology) error {
	// TODO Should this actually be prepend? Aka service post deploy THEN global post deploy
	err := appendDeployToGlobal("post-deploy.sh", topology)
	if err != nil {
		return err
	}
	return appendDeployToGlobal("pre-deploy.sh", topology)
}

func appendDeployToGlobal(fileName string, topology Topology) error {
	deployFile := path.Join(DeployFolder, BinFolder, fileName)
	if _, err := os.Stat(deployFile); err == nil {
		for _, serviceDef := range topology.serviceMetadata {
			servicePostDeployFile := path.Join(DeployFolder, serviceDef.Name, BinFolder, fileName)
			if _, err := os.Stat(servicePostDeployFile); err == nil {
				content, err := readTextFile(servicePostDeployFile)
				if err != nil {
					return err
				}
				content = strings.ReplaceAll(content, "#!/usr/bin/env bash\n", "")
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
		tmp := strings.ReplaceAll(templateFilePath, ServicesFolder, DeployFolder)
		tmp1 := strings.ReplaceAll(tmp, TemplateExt, "")
		resultFilePath := strings.ReplaceAll(tmp1, "~", fmt.Sprintf("-%s", nodeId(idx)))
		err = appendToFile(resultFilePath, res)
		if err != nil {
			return err
		}
	}
	return nil
}

type FileScanningContext struct {
	rootPath              string
	pathFragment          string
	includingPathFragment bool
	extension             string
}

type RenderTemplate func(templateFilePath string) (string, error)

func withScanningContext(ctx FileScanningContext, render RenderTemplate) ([]string, error) {
	var res []string
	err := filepath.Walk(ctx.rootPath, func(templateFilePath string, handle os.FileInfo, err error) error {
		nameMatches := ctx.includingPathFragment && strings.Contains(templateFilePath, ctx.pathFragment) ||
			!ctx.includingPathFragment && !strings.Contains(templateFilePath, ctx.pathFragment)
		if handle != nil && !handle.IsDir() && nameMatches && path.Ext(handle.Name()) == ctx.extension {
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
		if handle != nil && !handle.IsDir() && path.Ext(handle.Name()) == TemplateExt {
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
