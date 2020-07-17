package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
)

const TopologyFolder = "."                   // Where everything gets picked up from
const TopologyFile = "topology.txt"          // Definition file
const TopologyConfigFile = "topology.config" // Config file
const ServiceConfigFile = "service.config"   // Service config file
const ServicesFolder = "services"            // Where service folders live
const DeployFolder = "deploy"                // Where everything ends up, eventually
const BinFolder = "bin"                      // Where shell scripts live
const InheritRootFolder = "topology"         // Special folder for topology inheritance pack
const VendorFolder = ".lazy_vendor"          // Where inherited packs live
const TemplateExt = ".tmpl"                  // Everything with this extension gets rendered
const DefaultFileMode = os.FileMode(0644)    // Rendered files have these access rights

func topologyFile() string {
	return path.Join(TopologyFolder, TopologyFile)
}

func topologyConfigFile() string {
	return path.Join(TopologyFolder, TopologyConfigFile)
}

func serviceConfigFilePath(serviceName string) string {
	return path.Join(ServicesFolder, serviceName, ServiceConfigFile)
}

func serviceDir(serviceName string) string {
	return path.Join(servicesDir(), serviceName)
}

func servicesDir() string {
	return path.Join(TopologyFolder, ServicesFolder)
}

func deployDir() string {
	return path.Join(TopologyFolder, DeployFolder)
}

func deployFilePath(filePath string) string {
	return path.Join(DeployFolder, filePath)
}

func inheritRootDir() string {
	return path.Join(VendorFolder, InheritRootFolder)
}

func inheritServiceDir(serviceName string) string {
	return path.Join(VendorFolder, serviceName)
}

func inheritTopologyConfigFile() string {
	return path.Join(inheritRootDir(), TopologyConfigFile)
}

func inheritServiceConfigFile(serviceName string) string {
	return path.Join(inheritServiceDir(serviceName), ServiceConfigFile)
}

func stackFilePath(stackName string) string {
	return path.Join(deployDir(), DefaultSwarmDeployFolder, fmt.Sprintf("%s.yml", stackName))
}

func topologyJsonFile() string {
	return path.Join(deployDir(), "topology.json")
}

func rootInheritBinFolder() string {
	return path.Join(inheritRootDir(), BinFolder)
}

func MkDirs(path string) error {
	cmd := exec.Command("mkdir", "-p", path)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("mkdirs failed: %s, %w", path, err)
	}
	return nil
}

func CopyDir(source string, dest string) error {
	cmd := exec.Command("cp", "-R", source, dest)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("copy dirs failed: %s -> %s, %w", source, dest, err)
	}
	return err
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

type GitCache struct {
	cache map[string]string
}

func NewGitCache() *GitCache {
	return &GitCache{cache: map[string]string{}}
}

func (gitCache GitCache) cleanup() {
	for _, tempFolder := range gitCache.cache {
		err := os.RemoveAll(tempFolder)
		if err != nil {
			log.Fatal(fmt.Sprintf("git temp folder cleanup failed: %v", err))
		}
	}
}

func (gitCache GitCache) fetch(sourceSpec string, destFolder string) error {
	// if already there, don't clone
	var pathInfo, err = os.Stat(destFolder)
	// We continue only if it's not already there
	if !os.IsNotExist(err) && pathInfo.IsDir() {
		return nil
	}
	sourceDef := parseSourceDef(sourceSpec)
	repoKey := fmt.Sprintf("%s_%s", sourceDef.gitUrl, sourceDef.branch)
	var tempDir = gitCache.cache[repoKey]
	if tempDir == "" {
		tempDir, err = ioutil.TempDir("", "lazy")
		if err != nil {
			return err
		}
		cmd := exec.Command("git", "clone", "-b", sourceDef.branch, "--single-branch", "--depth", "1", sourceDef.gitUrl, tempDir)
		log.Println(fmt.Sprintf("cloning for '%s'... %v", sourceDef.subFolder, sourceDef.gitUrl))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("git clone failed: %s, %s, %w", sourceDef.gitUrl, sourceDef.branch, err)
		}
		// cleanup first
		err = os.RemoveAll(destFolder)
		if err != nil {
			return err
		}
		err = MkDirs(VendorFolder)
		if err != nil {
			return err
		}
		gitCache.cache[repoKey] = tempDir
	}
	err = CopyDir(path.Join(tempDir, sourceDef.subFolder), destFolder)
	return err
}
