# Raspberry Pi Golang MQTT node for Homeassistant

This is my take on implementing a "switch" type MQTT node for Homeassistant running on
a Raspberry Pi Zero (2), it should be easy to implement other node types by changing the presentation message

Configuration is done trough /etc/nod_conf.yaml file which should have the below fields:

```yaml
nodename: "MySwitch"
nodeid: "sw01"
nodetype: "switch"
mqtt_srv: "mqtt-server.example.com:1883"
mqtt_user: "switch-node"
mqtt_pass: "very_secret_password"

```

nodetype can be set to all supported types by Homeassistant but you will need to handle the correct
message generation, right now I only tested switch. 
