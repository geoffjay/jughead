package sites

import (
	"sync"
)

type SiteManager struct {
	sites map[string]*Site
}

var lock = &sync.Mutex{}
var instance *SiteManager

func GetSiteManager() *SiteManager {
	if instance == nil {
		lock.Lock()
		defer lock.Unlock()
		if instance == nil {
			instance = &SiteManager{
				sites: make(map[string]*Site),
			}
		}
	}

	return instance
}

func (sm *SiteManager) RegisterSite(path string, site *Site) {
	sm.sites[path] = site
}

func (sm *SiteManager) GetSite(path string) *Site {
	return sm.sites[path]
}

func (sm *SiteManager) Sites() map[string]*Site {
	return sm.sites
}
