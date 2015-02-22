package main

import (
	"encoding/json"
	"fmt"
	"math"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/devices"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-ninja/model"
	// "github.com/ninjasphere/go-ninja/support"
	"github.com/chrisn-au/go-limitless"
)

var info = ninja.LoadModuleInfo("./package.json")


type LimitlessDriver struct {
	log       *logger.Logger
	config    *LimitLessDriverConfig
	conn      *ninja.Connection
	client    *limitless.Client
	sendEvent func(event string, payload interface{}) error
}

func NewLimitlessDriver() {
	d := &LimitlessDriver{
		log:    logger.GetLogger(info.Name),
		client: limitless.NewClient(),
	}

	conn, err := ninja.Connect(info.ID)
	if err != nil {
		d.log.Fatalf("Failed to connect to MQTT: %s", err)
	}

	err = conn.ExportDriver(d)

	if err != nil {
		d.log.Fatalf("Failed to export driver: %s", err)
	}

	go func() {

		sub := d.client.Subscribe()

		for {

			event := <-sub.Events

			switch bulb := event.(type) {
			case *Limitless.Bulb:
				if isUnique(bulb) {
					d.log.Infof("creating new light")
					_, err := d.newLight(bulb)
					if err != nil {
						d.log.HandleErrorf(err, "Error creating light instance")
					}
					seenlights = append(seenlights, bulb) //TODO remove bulbs that haven't been seen in a while?
					err = d.client.GetBulbState(bulb)

					if err != nil {
						d.log.Warningf("unable to intiate bulb state request %s", err)
					}
				}
			default:
				d.log.Infof("Event %v", event)
			}

		}

	}()

	d.conn = conn
}

type LimitlessDriverConfig struct {
}

func (d *LimitlessDriver) Start(config *LimitlessDriverConfig) error {
	d.log.Infof("Starting with config %v", config)
	d.config = config

	err := d.client.StartDiscovery()
	if err != nil {
		err = fmt.Errorf("Failed to discover bulbs : %s", err)
	}
	return err
}

func (d *LimitlessDriver) Stop() error {
	return nil
}

func (d *LimitlessDriver) GetModuleInfo() *model.Module {
	return info
}

func (d *LimitlessDriver) SetEventHandler(sendEvent func(event string, payload interface{}) error) {
	d.sendEvent = sendEvent
}

//---------------------------------------------------------------[Bulb]----------------------------------------------------------------

func (d *LimitlessDriver) newLight(bulb *Limitless.Bulb) (*devices.LightDevice, error) { //TODO cut this down!

	name := bulb.GetLabel()

	d.log.Infof("Making light with ID: %s Label: %s", bulb.GetLimitlessAddress(), name)

	light, err := devices.CreateLightDevice(d, &model.Device{
		NaturalID:     bulb.GetLimitlessAddress(),
		NaturalIDType: "Limitless",
		Name:          &name,
		Signatures: &map[string]string{
			"ninja:manufacturer": "Limitless",
			"ninja:productName":  "Limitless Bulb",
			"ninja:productType":  "Light",
			"ninja:thingType":    "light",
		},
	}, d.conn)

	if err != nil {
		d.log.FatalError(err, "Could not create light device")
	}

	light.ApplyOnOff = func(state bool) error {
		var err error
		if state {
			err = d.client.LightOn(bulb)
		} else {
			err = d.client.LightOff(bulb)
		}
		if err != nil {
			return fmt.Errorf("Failed to set on-off state: %s", err)
		}
		return nil
	}

	
	bulb.SetStateHandler(buildStateHandler(d, bulb, light))

	if err := light.EnableOnOffChannel(); err != nil {
		d.log.FatalError(err, "Could not enable Limitless on-off channel")
	}
/*	
	if err := light.EnableBrightnessChannel(); err != nil {
		d.log.FatalError(err, "Could not enable Limitless brightness channel")
	}
	if err := light.EnableColorChannel("temperature", "hue"); err != nil {
		d.log.FatalError(err, "Could not enable Limitless color channel")
	}
	if err := light.EnableTransitionChannel(); err != nil {
		d.log.FatalError(err, "Could not enable Limitless transition channel")
	}
*/
	return light, nil
}

func buildStateHandler(driver *LimitlessDriver, bulb *Limitless.Bulb, light *devices.LightDevice) Limitless.StateHandler {

	return func(bulbState *Limitless.BulbState) {

		jsonState, _ := json.Marshal(bulbState)
		driver.log.Debugf("Incoming state: %s", jsonState)

		state := &devices.LightDeviceState{}

		onOff := int(bulbState.Power) > 0
		state.OnOff = &onOff

		brightness := float64(bulbState.Brightness) / math.MaxUint16
		state.Brightness = &brightness

		color := &channels.ColorState{}
		if bulbState.Saturation == 0 {
			color.Mode = "temperature"

			temperature := float64(bulbState.Kelvin)
			color.Temperature = &temperature

		} else {
			color.Mode = "hue"

			hue := float64(bulbState.Hue) / float64(math.MaxUint16)
			color.Hue = &hue

			saturation := float64(bulbState.Saturation) / float64(math.MaxUint16)
			color.Saturation = &saturation
		}

		state.Color = color

		light.SetLightState(state)
	}
}

//---------------------------------------------------------------[Utils]----------------------------------------------------------------

var seenlights []*Limitless.Bulb

func isUnique(newbulb *Limitless.Bulb) bool {
	ret := true
	for _, bulb := range seenlights {
		if bulb.LimitlessAddress == newbulb.LimitlessAddress {
			ret = false
		}
	}
	return ret
}
