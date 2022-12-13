package main

import (
	"github.com/percona/mongodb_exporter/exporter"
	"sync"
	"time"
)

type cacheExporters struct {
	mp    map[string]*multiExporter
	mutex *sync.RWMutex
}

func InitCacheExporter() *cacheExporters {
	return &cacheExporters{
		mp:    make(map[string]*multiExporter),
		mutex: &sync.RWMutex{},
	}
}

func (c *cacheExporters) Get(key string) (*multiExporter, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	result, found := c.mp[key]
	return result, found
}

func (c *cacheExporters) Set(key string, value *multiExporter) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.mp[key] = value
}

func (c *cacheExporters) Count() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.mp)
}

type multiExporter struct {
	exp        *exporter.Exporter
	createTime time.Time
	updateTime time.Time
}

func NewMultiExporter(opts *exporter.Opts) *multiExporter {
	return &multiExporter{
		exp:        exporter.New(opts),
		createTime: time.Now(),
		updateTime: time.Now(),
	}
}

func (m *multiExporter) SetUpdateTime(time time.Time) {
	m.updateTime = time
}
