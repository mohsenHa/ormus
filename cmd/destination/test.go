package main

import (
	"encoding/json"
	"fmt"
	"github.com/ormushq/ormus/config"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/newimplementation/channel"
	rbbitmqadapter "github.com/ormushq/ormus/destination/newimplementation/channel/adapter/rabbitmq"
	"github.com/ormushq/ormus/destination/newimplementation/channelmanager"
	"sync"
	"time"
)

//Crate adapter
//Create channel with adapter

func main() {
	done := make(chan bool)
	wg := sync.WaitGroup{}
	destinationConfig := config.C().Destination

	name := "test"

	channelManager := channelmanager.New(done, &wg)
	//simpleChannelAdapter := simple.New(done, &wg)
	channelAdapter := rbbitmqadapter.New(done, &wg, destinationConfig.RabbitmqConnection)
	channelAdapter.NewChannel(name, channel.BothMode, 100, 5)

	//channelManager.RegisterChannelAdapter("simple", simpleChannelAdapter)
	channelManager.RegisterChannelAdapter("rabbitmq", channelAdapter)

	go func() {
		c, _ := channelManager.GetOutputChannel("rabbitmq", name)
		println(c)
		for {
			select {
			case msg := <-c:
				fmt.Println(msg)
			default:
				continue
			}
		}
	}()
	c, _ := channelManager.GetInputChannel("rabbitmq", name)
	println(c)
	if j, err := json.Marshal(taskentity.Task{}); err == nil {
		c <- j
	} else {
		panic(err)
	}

	time.Sleep(time.Second * 2)
}
