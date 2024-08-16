/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package libvirt

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/beevik/etree"
	"github.com/digitalocean/go-libvirt"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

const (
	procPath            string = "/proc/%d/cgroup"
	maxCacheSize        int    = 1000
	cacheRemoveElements int    = 100
)

var (
	cacheExist        = map[uint64]string{}
	cacheNotExist     = map[uint64]bool{}
	regexFindVMIDPath = regexp.MustCompile(`machine-qemu.*\.scope`)
)

func GetVMID(pid uint64) (string, error) {
	fileName := fmt.Sprintf(procPath, pid)
	return getVMID(pid, fileName)
}

func getVMID(pid uint64, fileName string) (string, error) {
	if _, exist := cacheExist[pid]; exist {
		return cacheExist[pid], nil
	}
	if _, exist := cacheNotExist[pid]; exist {
		return "", fmt.Errorf("pid %d is not in a VM", pid)
	}

	// Read the file
	fileContents, err := os.ReadFile(fileName)
	if err != nil {
		addToNotExistCache(pid)
		return "", err
	}

	content := regexFindVMIDPath.FindAllString(string(fileContents), -1)
	if len(content) == 0 {
		addToNotExistCache(pid)
		return utils.EmptyString, fmt.Errorf("pid %d does not have vm ID", pid)
	}
	vmID := content[0]
	vmID = strings.ReplaceAll(vmID, "\\x2d", "-")
	vmID = strings.ReplaceAll(vmID, ".scope", "")

	if config.GetLibvirtMetadataURI() != "" {
		// NOTE: use VM Metadata instead
		if metaID, err := getMetadataVMID(vmID); metaID != "" && err == nil {
			vmID = metaID
		}
	}
	addVMIDToCache(pid, vmID)

	return vmID, nil
}

func addVMIDToCache(pid uint64, id string) {
	if len(cacheExist) >= maxCacheSize {
		counter := cacheRemoveElements
		// Remove elements from the cache
		for k := range cacheExist {
			delete(cacheExist, k)
			if counter == 0 {
				break
			}
			counter--
		}
	}
	cacheExist[pid] = id
}

func addToNotExistCache(pid uint64) {
	if len(cacheNotExist) >= maxCacheSize {
		counter := cacheRemoveElements
		// Remove elements from the cache
		for k := range cacheNotExist {
			delete(cacheNotExist, k)
			if counter == 0 {
				break
			}
			counter--
		}
	}
	cacheNotExist[pid] = false
}

func getMetadataVMID(vmid string) (metaVmid string, err error) {
	metadataURI := config.GetLibvirtMetadataURI()
	domainName := ""
	metaVmid = ""

	parts := strings.Split(vmid, "-")
	if len(parts) < 4 {
		return metaVmid, fmt.Errorf("the vmid provided does not match machine-qemu-number-name : %v", vmid)
	}

	domainName = strings.Join(parts[3:], "-")

	uri, _ := url.Parse(string(libvirt.QEMUSystem))
	l, err := libvirt.ConnectToURI(uri)
	if err != nil {
		return metaVmid, fmt.Errorf("libvirt failed to connect: %w", err)
	}

	defer func() {
		e := l.Disconnect()
		if err == nil {
			err = e
		}
	}()

	d, err := l.DomainLookupByName(domainName)
	if err != nil {
		return metaVmid, fmt.Errorf("libvirt Domainlookup fail %w", err)
	}

	metaURI := libvirt.OptString{metadataURI}
	// Domain Metadata Element = 0x2
	metaXML, _ := l.DomainGetMetadata(d, 0x2, metaURI, 0)

	doc := etree.NewDocument()
	if err := doc.ReadFromString(metaXML); err != nil {
		return metaVmid, fmt.Errorf("failed to read metadata XML: %w", err)
	}

	token := config.GetLibvirtMetadataToken()
	nameElement := doc.FindElement(fmt.Sprintf("//%s", token))
	if nameElement == nil {
		return metaVmid, fmt.Errorf("no <%s> element found", token)
	}
	return nameElement.Text(), nil
}
