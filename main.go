package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/gokrazy/gokrazy"
	"github.com/ilyakaznacheev/cleanenv"
	"periph.io/x/conn/v3/gpio"
	// "periph.io/x/conn/v3/gpio/gpioreg"
	host "periph.io/x/host/v3"
	"periph.io/x/host/v3/rpi"
)

type Cfg struct {
	NodeName  string `yaml:"nodename"`
	NodeId    string `yaml:"nodeid"`
	NodeType  string `yaml:"nodetype"`
	Mqtt_Srv  string `yaml:"mqtt_srv"`
	Mqtt_User string `yaml:"mqtt_user"`
	Mqtt_Pass string `yaml:"mqtt_pass"`
}

type deviceStruct struct {
	Identifiers []string `json:"identifiers"`
	Name        string   `json:"name"`
}

type presentPayload struct {
	Name          string       `json:"name"`
	DeviceClass   string       `json:"device_class"`
	StateTopic    string       `json:"state_topic"`
	CommandTopic  string       `json:"command_topic"`
	UniqueId      string       `json:"unique_id"`
	Device        deviceStruct `json:"device"`
	PayloadOn     string       `json:"payload_on"`
	PayloadOff    string       `json:"payload_off"`
	ValueTemplate string       `json:"value_template"`
}

type statePayload struct {
	State string `json:"state"`
}

var cfg Cfg
var online bool
var state bool = false

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Println("Connected")
	// sub(client)
	online = true
	var settopic string = fmt.Sprintf("homeassistant/%s/%s/set", cfg.NodeType, cfg.NodeId)
	log.Println("Subscribing to", settopic)
	sub(client, settopic)
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Printf("Connect lost: %v", err)
	online = false
	setRelay("off")
}

var reconnHandler mqtt.ReconnectHandler = func(client mqtt.Client, opts *mqtt.ClientOptions) {
	log.Println("Reconnecting:")
	// sub(client)
	var settopic string = fmt.Sprintf("homeassistant/%s/%s/set", cfg.NodeType, cfg.NodeId)
	log.Println("Resubscribing to", settopic)
	sub(client, settopic)
}

func readConf(file string, cfg *Cfg) error {
	err := cleanenv.ReadConfig(file, cfg)
	if err != nil {
		log.Printf("Error getting config: %s", err.Error())
		return err
	}
	return nil
}

func onMsg(client mqtt.Client, msg mqtt.Message) {

	var statetopic string = fmt.Sprintf("homeassistant/%s/%s/state", cfg.NodeType, cfg.NodeId)
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	log.Print("Sending back status: %s", msg.Payload())
	if err := setRelay(string(msg.Payload())); err != nil {
		log.Fatal(err)
	}
	pubState(client, statetopic, string(msg.Payload()))
}

func presentNode(client mqtt.Client, cfg *Cfg, statetopic string, cmdtopic string, conftopic string) {
	log.Println("Presenting myself ...")
	msg := presentPayload{
		Name:          cfg.NodeName,
		DeviceClass:   "switch",
		StateTopic:    statetopic,
		CommandTopic:  cmdtopic,
		UniqueId:      cfg.NodeId,
		PayloadOn:     "on",
		PayloadOff:    "off",
		ValueTemplate: "{{ value_json.state }}",
		Device: deviceStruct{
			Identifiers: []string{cfg.NodeName},
			Name:        cfg.NodeId,
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Print(err.Error())
	}
	publish(client, conftopic, payload)
	time.Sleep(500 * time.Millisecond)
	if state {

		pubState(client, statetopic, "on")
	} else {
		pubState(client, statetopic, "off")
	}
	log.Println("Done presenting.")
}

func pubState(client mqtt.Client, topic string, cmd string) {
	state := statePayload{
		State: cmd,
	}
	msg, err := json.Marshal(state)
	if err != nil {
		log.Print(err.Error())
	}
	publish(client, topic, msg)
}

func initClient(broker, id, usr, pass string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(id)
	opts.SetUsername(usr)
	opts.SetPassword(pass)
	opts.SetDefaultPublishHandler(onMsg)
	opts.SetConnectTimeout(1 * time.Second)
	// opts.SetAutoAckDisabled()
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	opts.SetKeepAlive(10 * time.Second)
	// opts.AutoReconnect = true
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	return client, nil
}

func sub(client mqtt.Client, topic string) {
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	log.Printf("Subscribed to topic %s", topic)
}

func publish(client mqtt.Client, topic string, msg []byte) {
	token := client.Publish(topic, 0, false, msg)
	token.Wait()
	log.Printf("Sent %s to topic %s", string(msg), topic)
	// time.Sleep(time.Second)
}

func setRelay(cmd string) error {
	log.Printf("Loading periph.io drivers")
	// Load periph.io drivers:
	if _, err := host.Init(); err != nil {
		return err
	}
	log.Printf("Toggling GPIO")
	if cmd == "on" {
		if err := rpi.P1_40.Out(gpio.Low); err != nil {
			log.Fatal(err)
		}
		state = true
	} else {
		if err := rpi.P1_40.Out(gpio.High); err != nil {
			log.Fatal(err)
		}
		state = false
	}
	return nil
}

func main() {
	represent := time.NewTicker(5 * time.Minute)
	// wait for ntp to be set
	gokrazy.WaitForClock()
	err := readConf("/etc/node_conf.yaml", &cfg)
	if err != nil {
		log.Fatal("Error getting config.")
	}
	var conftopic string = fmt.Sprintf("homeassistant/%s/%s/config", cfg.NodeType, cfg.NodeId)
	var statetopic string = fmt.Sprintf("homeassistant/%s/%s/state", cfg.NodeType, cfg.NodeId)
	var settopic string = fmt.Sprintf("homeassistant/%s/%s/set", cfg.NodeType, cfg.NodeId)
	client, err := initClient(cfg.Mqtt_Srv, cfg.NodeName, cfg.Mqtt_User, cfg.Mqtt_Pass)
	if err != nil {
		log.Fatal("Error creating MQTT client %s", err.Error())
	}
	// initial presentation
	presentNode(client, &cfg, statetopic, settopic, conftopic)
	sub(client, settopic)
	// run present node in a subroutine in case hass forgets me
	go func() {
		for {
			select {
			case <-represent.C:
				presentNode(client, &cfg, statetopic, settopic, conftopic)
			}
		}
	}()
	for {
		time.Sleep(1000 * time.Millisecond)
	}
}
