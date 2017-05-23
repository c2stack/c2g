package main

import (
	"flag"

	"github.com/c2stack/c2g/conf"
	"github.com/c2stack/c2g/examples/garage"
	"github.com/c2stack/c2g/meta"
	"github.com/c2stack/c2g/restconf"
)

var startup = flag.String("startup", "startup.json", "startup configuration file.")

func main() {
	flag.Parse()

	app := garage.NewGarage()

	// Where to looks for yang files, this tells library to use these
	// two relative paths.  StreamSource is an abstraction to data sources
	// that might be local or remote or combinations of all the above.
	uiPath := &meta.FileStreamSource{Root: "../web"}

	// notice the garage doesn't need yang for car.  it will get
	// that from proxy, that will in turn get it from car node, having
	// said that, if it does find yang locally, it will use it
	yangPath := meta.MultipleSources(
		&meta.FileStreamSource{Root: ".."},
		&meta.FileStreamSource{Root: "../../../yang"},
	)

	device := conf.NewLocalDeviceWithUi(yangPath, uiPath)
	chkErr(device.Add("garage", garage.Node(app)))

	// Standard management modules
	chkErr(device.Add("ietf-yang-library", conf.LocalDeviceYangLibNode(device)))

	callHome := conf.NewCallHome(yangPath, restconf.NewInsecureClientByHostAndPort)
	chkErr(device.Add("call-home", conf.CallHomeNode(callHome)))

	mgmt := restconf.NewManagement(device)
	chkErr(device.Add("restconf", restconf.Node(mgmt)))

	chkErr(device.ApplyStartupConfigFile(*startup))

	// wait for cntrl-c...
	select {}
}

func chkErr(err error) {
	if err != nil {
		panic(err)
	}
}
