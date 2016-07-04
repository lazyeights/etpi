package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/lazyeights/etpi"

	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/service"
)

type SecuritySystem struct {
	*accessory.Accessory
	Security *service.SecuritySystem
}

var acc *SecuritySystem
var panel etpi.Panel
var c mqtt.Client

var pwd string
var code string
var mqttAddr string
var etpiAddr string
var numPartitions int
var numZones int

const (
	SecuritySystemTargetStateStayArm = 0
	SecuritySystemTargetStateAwayArm = 1
	SecuritySystemTargetStateDisarm  = 3
)

const endPoints = `
    States (read-only, retained):
    etpi/[addr]/partition/[number]/state

    Commands (write-only):
    etpi/[addr]/partition/1/command/arm_stay
    etpi/[addr]/partition/1/command/arm_away
    etpi/[addr]/partition/1/command/arm_disarm
`

func init() {
	flag.StringVar(&pwd, "pwd", "user", "the password which is the same password to log into the Envisalink's local web page")
	flag.StringVar(&code, "code", "12345", "the user code that will be supplied to the security panel (e.g., to arm/disarm)")
	flag.StringVar(&etpiAddr, "etpi", "localhost:4025", "Envisalink 4 address (typically on port 4025)")
	flag.StringVar(&mqttAddr, "mqtt", "localhost:1883", "MQTT broker address (typically on port 1883)")
	flag.IntVar(&numPartitions, "partitions", 1, "number of partitions installed")
	flag.IntVar(&numZones, "zones", 4, "number of zones installed")
}

func main() {
	flag.Parse()
	fmt.Println("mqttepi version 0.0.0 type \"-h\" for help")

	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	panel = etpi.NewPanel()
	panel.OnZoneEvent(handleZone)
	panel.OnPartitionEvent(handlePartition)
	panel.OnKeypadEvent(handleKeypad)

	// Connect to MQTT broker
	addr := fmt.Sprintf("tcp://%s", mqttAddr)
	opts := mqtt.NewClientOptions().AddBroker(addr)
	c = mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("error: could not connect to MQTT broker at", mqttAddr)
		fmt.Println("error:", token.Error())
		os.Exit(1)
	}

	// Connect to Envisalink
	if err := panel.Connect(etpiAddr, pwd, code); err != nil {
		fmt.Println("error: could not connect to Envisalink at", err)
		os.Exit(1)
	}
	defer panel.Disconnect()

	c.Subscribe("etpi/partition/1/command/arm_stay", 2, mqtt.MessageHandler(func(c mqtt.Client, msg mqtt.Message) {
		log.Println("=== ARM STAY ===")
		updateTargetState(SecuritySystemTargetStateStayArm)
	}))

	c.Subscribe("etpi/partition/1/command/arm_away", 2, mqtt.MessageHandler(func(c mqtt.Client, msg mqtt.Message) {
		log.Println("=== ARM AWAY ===")
		updateTargetState(SecuritySystemTargetStateAwayArm)
	}))

	c.Subscribe("etpi/partition/1/command/disarm", 2, mqtt.MessageHandler(func(c mqtt.Client, msg mqtt.Message) {
		log.Println("=== DISARM ===")
		updateTargetState(SecuritySystemTargetStateDisarm)
	}))

	// Update initial status
	status := panel.Status()
	for i := 0; i < numPartitions; i++ {
		handlePartition(i+1, status.Partition[i])
	}
	for i := 0; i < numZones; i++ {
		handleZone(i+1, status.Zone[i])
	}

	<-sigch
}

func handlePartition(partition int, status etpi.PartitionStatus) {
	topic := fmt.Sprintf("etpi/partition/%d/state", partition)
	c.Publish(topic, 2, false, status.String())
}

func handleZone(zone int, status etpi.ZoneStatus) {
	topic := fmt.Sprintf("etpi/zone/%d/state", zone)
	c.Publish(topic, 2, false, status.String())
}

func handleKeypad(status etpi.KeypadStatus) {
}

func updateTargetState(state int) {
	switch state {
	case SecuritySystemTargetStateStayArm:
		panel.Arm(1, etpi.ArmStay)
	case SecuritySystemTargetStateAwayArm:
		panel.Arm(1, etpi.ArmAway)
	case SecuritySystemTargetStateDisarm:
		panel.Disarm(1)
	}
}
