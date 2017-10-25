// Craig Hesling
// September 9, 2017
//
// This OpenChirp service translates raw byte streams from devices
// into meaningful values
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/openchirp/framework"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	version string = "1.0"
)

const (
	// Set this value to true to have the service publish a service status of
	// "Running" each time it receives a device update event
	//
	// This could be used as a service alive pulse if enabled
	// Otherwise, the service status will indicate "Started" at the time the
	// service "Started" the client
	runningStatus = true
)

const (
	defaultDefaultType       = "int16"
	propertyDefaultType      = "Default Type"
	configIncomingFieldNames = "Incoming Field Names"
	configIncomingFieldTypes = "Incoming Field Types"
	configOutgoingFieldNames = "Outgoing Field Names"
	configOutgoingFieldTypes = "Outgoing Field Types"
	configAggregationDelay   = "Aggregation Delay"
	configEndianness         = "Endianness"
)

func run(ctx *cli.Context) error {
	/* Set logging level */
	log := logrus.New()
	log.SetLevel(logrus.Level(uint32(ctx.Int("log-level"))))

	log.Info("Starting Byte Translator Service ")

	/* Start framework service client */
	c, err := framework.StartServiceClientStatus(
		ctx.String("framework-server"),
		ctx.String("mqtt-server"),
		ctx.String("service-id"),
		ctx.String("service-token"),
		"Unexpected disconnect!")
	if err != nil {
		log.Error("Failed to StartServiceClient: ", err)
		return cli.NewExitError(nil, 1)
	}
	defer c.StopClient()
	log.Debug("Started service")

	/* Post service status indicating I am starting */
	err = c.SetStatus("Starting")
	if err != nil {
		log.Error("Failed to publish service status: ", err)
		return cli.NewExitError(nil, 1)
	}
	log.Debug("Published Service Status")

	/* Launch ByteTranslator */
	defaultType := c.GetProperty(propertyDefaultType)
	if len(defaultType) == 0 {
		defaultType = defaultDefaultType
	}
	bt, err := NewByteTranslator(c, log, defaultType)
	if err != nil {
		log.Fatal("Failed to parse service property " + propertyDefaultType)
		err = c.SetStatus("Service property " + propertyDefaultType + " invalid")
		if err != nil {
			log.Error("Failed to publish service status: ", err)
			return cli.NewExitError(nil, 1)
		}
	}
	log.Debug("Started ByteTranslator")

	/* Start service main device updates stream */
	log.Debug("Starting Device Updates Stream")
	updates, err := c.StartDeviceUpdatesSimple()
	if err != nil {
		log.Error("Failed to start device updates stream: ", err)
		return cli.NewExitError(nil, 1)
	}
	defer c.StopDeviceUpdates()

	/* Setup signal channel */
	log.Debug("Processing device updates")
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	/* Post service status indicating I started */
	err = c.SetStatus("Started")
	if err != nil {
		log.Error("Failed to publish service status: ", err)
		return cli.NewExitError(nil, 1)
	}
	log.Debug("Published Service Status")

	for {
		select {
		case update := <-updates:
			/* If runningStatus is set, post a service status as an alive msg */
			if runningStatus {
				err = c.SetStatus("Running")
				if err != nil {
					log.Error("Failed to publish service status: ", err)
					return cli.NewExitError(nil, 1)
				}
				log.Debug("Published Service Status")
			}

			logitem := log.WithFields(logrus.Fields{"type": update.Type, "deviceid": update.Id})

			switch update.Type {
			case framework.DeviceUpdateTypeRem:
				logitem.Info("Removing device")
				bt.RemoveDevice(update.Id)
			case framework.DeviceUpdateTypeUpd:
				logitem.Info("Removing device for update")
				bt.RemoveDevice(update.Id)
				fallthrough
			case framework.DeviceUpdateTypeAdd:
				logitem.Info("Adding device")
				deverr, runerr := bt.AddDevice(
					update.Id,
					update.Config[configIncomingFieldNames],
					update.Config[configIncomingFieldTypes],
					update.Config[configOutgoingFieldNames],
					update.Config[configOutgoingFieldTypes],
					update.Config[configEndianness],
					update.Config[configAggregationDelay],
				)
				if runerr != nil {
					// runtime error
					logitem.Error("Failed to add device: ", runerr)
					continue
				}
				if deverr != nil {
					// device config error
					logitem.Warn("Device config error: ", deverr)
					c.SetDeviceStatus(update.Id, deverr)
					continue
				}
				c.SetDeviceStatus(update.Id, "Success")
			}
		case sig := <-signals:
			log.WithField("signal", sig).Info("Received signal")
			goto cleanup
		}
	}

cleanup:

	log.Warning("Shutting down")
	err = c.SetStatus("Shutting down")
	if err != nil {
		log.Error("Failed to publish service status: ", err)
	}
	log.Info("Published service status")

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "example-service"
	app.Usage = ""
	app.Copyright = "See https://github.com/openchirp/example-service for copyright information"
	app.Version = version
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "framework-server",
			Usage:  "OpenChirp framework server's URI",
			Value:  "http://localhost:7000",
			EnvVar: "FRAMEWORK_SERVER",
		},
		cli.StringFlag{
			Name:   "mqtt-server",
			Usage:  "MQTT server's URI (e.g. scheme://host:port where scheme is tcp or tls)",
			Value:  "tls://localhost:1883",
			EnvVar: "MQTT_SERVER",
		},
		cli.StringFlag{
			Name:   "service-id",
			Usage:  "OpenChirp service id",
			EnvVar: "SERVICE_ID",
		},
		cli.StringFlag{
			Name:   "service-token",
			Usage:  "OpenChirp service token",
			EnvVar: "SERVICE_TOKEN",
		},
		cli.IntFlag{
			Name:   "log-level",
			Value:  4,
			Usage:  "debug=5, info=4, warning=3, error=2, fatal=1, panic=0",
			EnvVar: "LOG_LEVEL",
		},
	}
	app.Run(os.Args)
}
