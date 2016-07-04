package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/service"
	"github.com/eclipse/paho.mqtt.golang"
)

type SecuritySystem struct {
	*accessory.Accessory
	Security *service.SecuritySystem
}

var acc *SecuritySystem
var c mqtt.Client

var mqttAddr string

func init() {
	flag.StringVar(&mqttAddr, "mqtt", "localhost:1883", "MQTT broker address (typically on port 1883)")
}

func main() {
	flag.Parse()
	fmt.Println("hkmqtt version 0.0.0 type \"-h\" for help")

	info := accessory.Info{
		Name:         "Envisalink4",
		Manufacturer: "EyezOn",
	}

	acc = &SecuritySystem{
		Accessory: accessory.New(info, accessory.TypeAlarmSystem),
		Security:  service.NewSecuritySystem(),
	}

	acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateDisarmed)
	acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateDisarm)

	acc.AddService(acc.Security.Service)

	acc.Security.SecuritySystemTargetState.OnValueRemoteUpdate(updateTargetState)

	t, err := hc.NewIPTransport(hc.Config{Pin: "32191123"}, acc.Accessory)
	if err != nil {
		log.Fatal(err)
	}

	hc.OnTermination(func() {
		t.Stop()
	})

	// Connect to MQTT broker
	addr := fmt.Sprintf("tcp://%s", mqttAddr)
	opts := mqtt.NewClientOptions().AddBroker(addr)
	c = mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("error: could not connect to MQTT broker at", mqttAddr)
		fmt.Println("error:", token.Error())
		os.Exit(1)
	}

	c.Subscribe("etpi/partition/1/state", 2, mqtt.MessageHandler(func(c mqtt.Client, msg mqtt.Message) {
		handlePartition(1, string(msg.Payload()))
	}))

	t.Start()
}

func handlePartition(partition int, status string) {
	if partition != 1 {
		return
	}
	if acc == nil {
		return
	}
	switch status {
	case "DISARMED_READY",
		"DISARMED":
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateDisarmed)
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateDisarm)
	case "ARMED_AWAY",
		"ARMED_ZERO_ENTRY_AWAY":
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateAwayArm)
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateAwayArm)
	case "ARMED_STAY",
		"ARMED_ZERO_ENTRY_STAY":
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateStayArm)
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateStayArm)
	case "ALARM":
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateAlarmTriggered)
	}
}

func updateTargetState(state int) {
	switch state {
	case characteristic.SecuritySystemTargetStateStayArm:
		c.Publish("etpi/partition/1/command/arm_stay", 2, false, "")
	case characteristic.SecuritySystemTargetStateAwayArm:
		c.Publish("etpi/partition/1/command/arm_away", 2, false, "")
	case characteristic.SecuritySystemTargetStateNightArm:
		// TODO XXX: No direct mapping to night arm for TPI interface
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateStayArm)
		c.Publish("etpi/partition/1/command/arm_stay", 2, false, "")
	case characteristic.SecuritySystemTargetStateDisarm:
		c.Publish("etpi/partition/1/command/disarm", 2, false, "")
	}
}
