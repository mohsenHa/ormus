package channelmanager

import (
	"fmt"
	"github.com/ormushq/ormus/destination/newimplementation/channel"
	"sync"
)

type Manager struct {
	wg              *sync.WaitGroup
	done            <-chan bool
	channelAdapters map[string]channel.Adapter
}

const channelNotFound = "channel adapter not found: %v"

func New(done <-chan bool, wg *sync.WaitGroup) Manager {
	return Manager{
		done:            done,
		wg:              wg,
		channelAdapters: make(map[string]channel.Adapter),
	}
}

func (m Manager) RegisterChannelAdapter(name string, adapter channel.Adapter) {
	m.channelAdapters[name] = adapter
}

func (m Manager) GetInputChannel(adapterName string, channelName string) (chan<- []byte, error) {
	if ca, ok := m.channelAdapters[adapterName]; ok {
		return ca.GetInputChannel(channelName)
	}
	return nil, fmt.Errorf(channelNotFound, adapterName)
}
func (m Manager) GetOutputChannel(adapterName string, channelName string) (<-chan []byte, error) {
	if ca, ok := m.channelAdapters[adapterName]; ok {
		return ca.GetOutputChannel(channelName)
	}
	return nil, fmt.Errorf(channelNotFound, adapterName)
}
func (m Manager) GetMode(adapterName string, channelName string) (channel.Mode, error) {
	if ca, ok := m.channelAdapters[adapterName]; ok {
		return ca.GetMode(channelName)
	}
	return "", fmt.Errorf(channelNotFound, adapterName)
}
