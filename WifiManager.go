package main

import "github.com/ninjasphere/go-wireless/wpactl"

type WifiManager struct {
	Controller  *wpactl.WPAController
	stateChange []chan string
	ackPending  bool   // true if we need to wait for acknowledgment from app
	onAck       func() // optional function to execute once acnknowledgment of credentials received
}

const (
	WifiStateDisconnected = "disconnected"
	WifiStateConnected    = "connected"
	WifiStateInvalidKey   = "invalid_key"
)

func NewWifiManager(iface string) (*WifiManager, error) {
	ctl, err := wpactl.NewController(iface)
	if err != nil {
		return nil, err
	}

	manager := &WifiManager{}
	manager.stateChange = make([]chan string, 0)
	manager.Controller = ctl

	go manager.eventLoop()

	return manager, nil
}

func (m *WifiManager) WatchState() chan string {
	ch := make(chan string, 128)

	m.stateChange = append(m.stateChange, ch)

	return ch
}

func (m *WifiManager) UnwatchState(target chan string) {
	for i, c := range m.stateChange {
		if c == target {
			m.stateChange[i] = nil
		}
	}
}

func (m *WifiManager) emitState(state string) {
	for _, ch := range m.stateChange {
		if ch != nil {
			ch <- state
		}
	}
}

func (m *WifiManager) eventLoop() {
	for {
		event := <-m.Controller.EventChannel
		logger.Infof("eventLoop: %v", event)
		switch event.Name {
		case "CTRL-EVENT-DISCONNECTED":
			m.emitState(WifiStateDisconnected)
		case "CTRL-EVENT-CONNECTED":
			m.emitState(WifiStateConnected)
		case "CTRL-EVENT-SSID-TEMP-DISABLED":
			m.emitState(WifiStateInvalidKey)
		}
	}
}

func (m *WifiManager) Cleanup() {
	m.Controller.Cleanup()
}

func (m *WifiManager) SetCredentials(wifi_creds *WifiCredentials) bool {

	logger.Infof("SetCredentials: Setting credentials. ssid: %s - password length: %d", wifi_creds.SSID, len(wifi_creds.Key))

	m.ackPending = true
	WriteToFile("/etc/network/interfaces.d/wlan0", WLANInterfaceTemplate)

	states := m.WatchState()

	m.AddStandardNetwork(wifi_creds.SSID, wifi_creds.Key)
	m.Controller.ReloadConfiguration()

	success := true
	for {
		state := <-states
		logger.Infof("SetCredentials: Network state: %s", state)
		if state == WifiStateConnected {
			success = true
			break
		} else if state == WifiStateInvalidKey {
			m.ackPending = false
			success = false
			break
		}
	}

	m.UnwatchState(states)

	logger.Debugf("SetCredentials: Returning Success: %t", success)

	return success
}

func (m *WifiManager) WifiConfigured() (bool, error) {
	networks, err := m.Controller.ListNetworks()
	if err != nil {
		return false, nil
	}
	enabledNetworks := 0
	for _, network := range networks {
		result, _ := m.Controller.GetNetworkSetting(network.Id, "disabled")
		if result == "1" {
			continue
		}
		enabledNetworks++
	}
	return (enabledNetworks > 0), nil
}

func (m *WifiManager) DisableAllNetworks() error {
	networks, err := m.Controller.ListNetworks()
	if err != nil {
		return err
	}

	for _, network := range networks {
		m.Controller.DisableNetwork(network.Id)
	}

	return nil
}

func (m *WifiManager) AddStandardNetwork(ssid string, key string) error {
	i, err := m.Controller.AddNetwork()
	if err != nil {
		return err
	}
	// FIXME: handle errors for all of these!
	m.Controller.SetNetworkSettingString(i, "ssid", ssid)
	m.Controller.SetNetworkSettingString(i, "psk", key)
	m.Controller.SetNetworkSettingRaw(i, "scan_ssid", "1")
	m.Controller.SetNetworkSettingRaw(i, "key_mgmt", "WPA-PSK")
	m.Controller.SelectNetwork(i)
	m.Controller.SaveConfiguration()

	return nil
}

func (m *WifiManager) ConnectionAcknowledged() {
	if m.ackPending {
		m.ackPending = false
		if m.onAck != nil {
			m.onAck()
		}
	}
}

func (m *WifiManager) OnAcknowledgment(then func()) {
	if m.ackPending {
		m.onAck = then
	} else {
		then()
	}
}
