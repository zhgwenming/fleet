package machine

import (
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/docker/libcontainer/netlink"
	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/resource"
	"github.com/coreos/fleet/systemd"
)

const (
	machineIDPath = "/etc/machine-id"
)

func NewCoreOSMachine(static MachineState, sd *systemd.SystemdUnitManager) *CoreOSMachine {
	log.V(1).Infof("Created CoreOSMachine with static state %s", static)
	m := &CoreOSMachine{
		staticState: static,
		systemd:     sd,
	}
	return m
}

type CoreOSMachine struct {
	sync.RWMutex

	systemd      *systemd.SystemdUnitManager
	staticState  MachineState
	dynamicState *MachineState
}

func (m *CoreOSMachine) String() string {
	return m.State().ID
}

// State returns a MachineState object representing the CoreOSMachine's
// static state overlaid on its dynamic state at the time of execution.
func (m *CoreOSMachine) State() (state MachineState) {
	m.RLock()
	defer m.RUnlock()

	if m.dynamicState == nil {
		state = MachineState(m.staticState)
	} else {
		state = stackState(m.staticState, *m.dynamicState)
	}

	return
}

// Refresh updates the current state of the CoreOSMachine.
func (m *CoreOSMachine) Refresh() {
	m.RLock()
	defer m.RUnlock()

	m.dynamicState = m.currentState()
}

// PeriodicRefresh updates the current state of the CoreOSMachine at the
// interval indicated. Operation ceases when the provided channel is closed.
func (m *CoreOSMachine) PeriodicRefresh(interval time.Duration, stop chan bool) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-stop:
			log.V(1).Info("Halting CoreOSMachine.PeriodicRefresh")
			ticker.Stop()
			return
		case <-ticker.C:
			m.Refresh()
		}
	}
}

// currentState generates a MachineState object with the values read from
// the local system
func (m *CoreOSMachine) currentState() *MachineState {
	id := readLocalMachineID("/")
	publicIP := getLocalIP()
	// TODO(jonboulle): clarify failure behaviour when unable to retrieve resources/units
	totalResources, err := readLocalResources()
	if err != nil {
		log.Errorf("Error retrieving local resources: %v\n", err)
		totalResources = resource.ResourceTuple{}
	}
	units, err := m.systemd.Units()
	if err != nil {
		log.Errorf("Error retrieving local units: %v\n", err)
		units = []string{}
	}
	return &MachineState{
		ID:             id,
		PublicIP:       publicIP,
		Metadata:       make(map[string]string, 0),
		TotalResources: totalResources,
		LoadedUnits:    len(units),
	}
}

// IsLocalMachineID returns whether the given machine ID is equal to that of the local machine
func IsLocalMachineID(mID string) bool {
	return mID == readLocalMachineID("/")
}

func readLocalMachineID(root string) string {
	fullPath := filepath.Join(root, machineIDPath)
	id, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(id))
}

func getLocalIP() string {
	iface := getDefaultGatewayIface()
	if iface == nil {
		return ""
	}

	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		return ""
	}

	for _, addr := range addrs {
		// Attempt to parse the address in CIDR notation
		// and assert it is IPv4
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.To4() != nil {
			return ip.String()
		}
	}

	return ""
}

func getDefaultGatewayIface() *net.Interface {
	log.V(1).Infof("Attempting to retrieve IP route info from netlink")

	routes, err := netlink.NetworkGetRoutes()
	if err != nil {
		log.V(1).Infof("Unable to detect default interface: %v", err)
		return nil
	}

	if len(routes) == 0 {
		log.V(1).Infof("Netlink returned zero routes")
		return nil
	}

	for _, route := range routes {
		if route.Default {
			if route.Iface == nil {
				log.V(1).Infof("Found default route but could not determine interface")
			}
			log.V(1).Infof("Found default route with interface %v", route.Iface.Name)
			return route.Iface
		}
	}

	log.V(1).Infof("Unable to find default route")
	return nil
}
