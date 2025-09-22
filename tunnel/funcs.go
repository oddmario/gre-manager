package tunnel

import (
	"strings"
	"time"

	"github.com/go-ping/ping"
	"github.com/go-resty/resty/v2"
	"github.com/oddmario/tunnel-manager/logger"
	"github.com/oddmario/tunnel-manager/utils"
)

func (t *Tunnel) sendIPToTunHost(main_network_interface string, dynamic_ip_updater_api_listen_port, dynamic_ip_update_attempt_interval, dynamic_ip_update_timeout int) {
	var redoRouteAllTrafficThroughTunnel bool = false

	if t.ShouldRouteAllTrafficThroughTunnel && t.IsInitialised {
		gatewayIP, err := utils.Cmd("ip route show 0.0.0.0/0 dev "+main_network_interface+" | cut -d\\  -f3", true, true)
		if err == nil {
			gatewayIPString := strings.TrimSpace(utils.BytesToString(gatewayIP))

			utils.Cmd("ip route del default via "+t.TunHostTunnelIP+" metric 0", true, true)
			utils.Cmd("ip route del "+t.TunHostMainPublicIP+" via "+gatewayIPString+" dev "+main_network_interface+" onlink", true, true)

			redoRouteAllTrafficThroughTunnel = true
		}
	}

	for {
		external_ip, err := utils.GetExternalIP(t.TunHostMainPublicIP, dynamic_ip_update_timeout, dynamic_ip_updater_api_listen_port)

		if err != nil {
			logger.Warn("Unable to send the public IP address of the backend to the tunnel host. Retrying in " + utils.IToStr(dynamic_ip_update_attempt_interval) + " seconds...")

			time.Sleep(time.Duration(dynamic_ip_update_attempt_interval) * time.Second)

			continue
		}

		req, _ := resty.New().SetTimeout(time.Duration(dynamic_ip_update_timeout)*time.Second).R().
			SetHeader("X-Key", t.DynamicIPUpdaterKey).
			SetBody(map[string]interface{}{"new_ip": external_ip}).
			Post("http://" + t.TunHostMainPublicIP + ":" + utils.IToStr(dynamic_ip_updater_api_listen_port) + "/update_ip")

		if req.StatusCode() != 200 {
			logger.Warn("Unable to send the public IP address of the backend to the tunnel host. Retrying in " + utils.IToStr(dynamic_ip_update_attempt_interval) + " seconds...")

			time.Sleep(time.Duration(dynamic_ip_update_attempt_interval) * time.Second)

			continue
		}

		t.BackendServerPublicIP = external_ip

		break
	}

	if redoRouteAllTrafficThroughTunnel {
		gatewayIP, err := utils.Cmd("ip route show 0.0.0.0/0 dev "+main_network_interface+" | cut -d\\  -f3", true, true)
		if err == nil {
			gatewayIPString := strings.TrimSpace(utils.BytesToString(gatewayIP))

			utils.Cmd("echo 'nameserver 1.1.1.1' > /etc/resolv.conf", true, true)
			utils.Cmd("echo 'nameserver 1.0.0.1' >> /etc/resolv.conf", true, true)

			utils.Cmd("ip route add "+t.TunHostMainPublicIP+" via "+gatewayIPString+" dev "+main_network_interface+" onlink", true, true)
			utils.Cmd("ip route add default via "+t.TunHostTunnelIP+" metric 0", true, true)
		}
	}
}

func (t *Tunnel) Init(mode, main_network_interface string, dynamic_ip_updater_api_listen_port, dynamic_ip_update_attempt_interval, dynamic_ip_update_timeout, ping_interval, ping_timeout int) bool {
	if t.IsInitialised {
		logger.Warn("Failed to initialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": The tunnel has already been initialised. Ignoring tunnel initialisation.")

		return false
	}

	if utils.DoesNetworkInterfaceExist(t.TunnelInterfaceName) {
		logger.Warn("Failed to initialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": Tunnel interface already exists. Ignoring tunnel initialisation.")

		return false
	}

	if mode == "tunnel_host" {
		if t.BackendServerPublicIP == "DYNAMIC" {
			logger.Warn("Failed to initialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": The initial backend IP is `DYNAMIC` but no IP has been received from the backend yet. Ignoring tunnel initialisation.")

			return false
		}

		if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
			utils.Cmd("ip tunnel add "+t.TunnelInterfaceName+" mode "+t.TunnelDriver+" local "+t.TunHostMainPublicIP+" remote "+t.BackendServerPublicIP+" ttl 255 key "+utils.IToStr(t.TunnelKey), true, true)
			utils.Cmd("ip addr add "+t.TunHostTunnelIP+"/30 dev "+t.TunnelInterfaceName, true, true)
		}

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("ip link add "+t.TunnelInterfaceName+" type wireguard", true, true)
			utils.Cmd("ip addr add "+t.TunHostTunnelIP+"/24 dev "+t.TunnelInterfaceName, true, true)
			utils.Cmd("wg set "+t.TunnelInterfaceName+" private-key \""+t.WGPrivateKeyFilePath+"\"", true, true)
		}

		utils.Cmd("ip link set "+t.TunnelInterfaceName+" up", true, true)

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("wg set "+t.TunnelInterfaceName+" listen-port "+utils.IToStr(t.WGServerTunnelHostListenPort)+" peer "+t.WGBackendServerPubKey+" allowed-ips "+t.BackendServerTunnelIP+"/32 endpoint "+t.BackendServerPublicIP+":"+utils.IToStr(t.WGServerBackendServerListenPort)+" persistent-keepalive 25", true, true)
		}

		utils.Cmd("iptables-nft -A FORWARD -i "+t.TunnelInterfaceName+" -j ACCEPT", true, true)
		utils.Cmd("iptables-nft -A FORWARD -d "+t.BackendServerTunnelIP+" -m state --state NEW,ESTABLISHED,RELATED -j ACCEPT", true, true)
		utils.Cmd("iptables-nft -A FORWARD -s "+t.BackendServerTunnelIP+" -m state --state NEW,ESTABLISHED,RELATED -j ACCEPT", true, true)

		if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
			utils.Cmd("iptables-nft -t nat -A POSTROUTING -s "+t.TunnelGatewayIP+"/30 ! -o "+t.TunnelInterfaceName+" -j SNAT --to-source "+t.TunHostPublicIP, true, true)
		}

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("iptables-nft -t nat -A POSTROUTING -s "+t.TunnelGatewayIP+"/24 ! -o "+t.TunnelInterfaceName+" -j SNAT --to-source "+t.TunHostPublicIP, true, true)
		}

		if t.TunnelType == "full" {
			utils.Cmd("iptables-nft -t nat -A PREROUTING -d "+t.TunHostPublicIP+" -j DNAT --to-destination "+t.BackendServerTunnelIP, true, true)
		} else {
			for _, port := range t.SplitTunnelPorts {
				p := port["src_port"].(string)
				dp := port["dest_port"].(string)
				proto := port["proto"].(string)

				dstPort := ""

				if len(dp) > 0 {
					dstPort = ":" + dp
				}

				utils.Cmd("iptables-nft -t nat -A PREROUTING -d "+t.TunHostPublicIP+" -p "+proto+" -m "+proto+" --dport "+p+" -j DNAT --to-destination "+t.BackendServerTunnelIP+dstPort, true, true)
			}
		}
	}

	if mode == "backend_server" {
		var backendIsDynamicIP bool = false

		routingTableExists, err := rttablesCheck(t.TunnelRoutingTablesID, t.TunnelRoutingTablesName)
		if err != nil {
			logger.Warn("Failed to initialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": Routing table check failed -> " + err.Error() + ". Ignoring tunnel initialisation.")

			return false
		}

		if !routingTableExists {
			err := rttablesWrite(t.TunnelRoutingTablesID, t.TunnelRoutingTablesName)

			if err != nil {
				logger.Warn("Failed to initialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": Routing table write failed -> " + err.Error() + ". Ignoring tunnel initialisation.")

				return false
			}
		}

		if t.BackendServerPublicIP == "DYNAMIC" {
			backendIsDynamicIP = true

			t.sendIPToTunHost(main_network_interface, dynamic_ip_updater_api_listen_port, dynamic_ip_update_attempt_interval, dynamic_ip_update_timeout)
		}

		if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
			utils.Cmd("ip tunnel add "+t.TunnelInterfaceName+" mode "+t.TunnelDriver+" local "+t.BackendServerPublicIP+" remote "+t.TunHostMainPublicIP+" ttl 255 key "+utils.IToStr(t.TunnelKey), true, true)
			utils.Cmd("ip addr add "+t.BackendServerTunnelIP+"/30 dev "+t.TunnelInterfaceName, true, true)
		}

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("ip link add "+t.TunnelInterfaceName+" type wireguard", true, true)
			utils.Cmd("ip addr add "+t.BackendServerTunnelIP+"/24 dev "+t.TunnelInterfaceName, true, true)
			utils.Cmd("wg set "+t.TunnelInterfaceName+" private-key \""+t.WGPrivateKeyFilePath+"\"", true, true)
		}

		utils.Cmd("ip link set "+t.TunnelInterfaceName+" up", true, true)

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("wg set "+t.TunnelInterfaceName+" listen-port "+utils.IToStr(t.WGServerBackendServerListenPort)+" peer "+t.WGTunnelHostPubKey+" allowed-ips 0.0.0.0/0,::/0 endpoint "+t.TunHostMainPublicIP+":"+utils.IToStr(t.WGServerTunnelHostListenPort)+" persistent-keepalive 25", true, true)
		}

		if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
			utils.Cmd("ip rule add from "+t.TunnelGatewayIP+"/30 table "+t.TunnelRoutingTablesName, true, true)
		}

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("ip rule add from "+t.TunnelGatewayIP+"/24 table "+t.TunnelRoutingTablesName, true, true)
		}

		utils.Cmd("ip route add default via "+t.TunHostTunnelIP+" table "+t.TunnelRoutingTablesName, true, true)

		if t.ShouldRouteAllTrafficThroughTunnel {
			gatewayIP, err := utils.Cmd("ip route show 0.0.0.0/0 dev "+main_network_interface+" | cut -d\\  -f3", true, true)
			if err == nil {
				gatewayIPString := strings.TrimSpace(utils.BytesToString(gatewayIP))

				utils.Cmd("echo 'nameserver 1.1.1.1' > /etc/resolv.conf", true, true)
				utils.Cmd("echo 'nameserver 1.0.0.1' >> /etc/resolv.conf", true, true)

				utils.Cmd("ip route add "+t.TunHostMainPublicIP+" via "+gatewayIPString+" dev "+main_network_interface+" onlink", true, true)
				utils.Cmd("ip route add default via "+t.TunHostTunnelIP+" metric 0", true, true)
			}
		}

		if backendIsDynamicIP {
			go func() {
				for {
					for i := range 4 {
						pinger, pingerErr := ping.NewPinger(t.TunHostTunnelIP)
						if pingerErr != nil {
							time.Sleep(1 * time.Second)

							continue
						}

						pinger.RecordRtts = false
						pinger.Count = 1
						pinger.Timeout = time.Duration(ping_timeout) * time.Second

						pingerRunErr := pinger.Run()
						if pingerRunErr != nil {
							time.Sleep(1 * time.Second)

							continue
						}

						stats := pinger.Statistics()

						if stats.PacketLoss != 100 {
							break
						} else {
							if i >= 3 {
								logger.Info("Tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": The tunnel is unable to connect to the tunnel host, a network change might have occurred. Attempting to check for any dynamic IP changes...")

								t.sendIPToTunHost(main_network_interface, dynamic_ip_updater_api_listen_port, dynamic_ip_update_attempt_interval, dynamic_ip_update_timeout)

								if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
									utils.Cmd("ip tunnel change "+t.TunnelInterfaceName+" mode "+t.TunnelDriver+" local "+t.BackendServerPublicIP+" remote "+t.TunHostMainPublicIP+" ttl 255 key "+utils.IToStr(t.TunnelKey), true, true)
								}

								if t.TunnelDriver == "wireguard" {
									utils.Cmd("wg set "+t.TunnelInterfaceName+" listen-port "+utils.IToStr(t.WGServerBackendServerListenPort)+" peer "+t.WGTunnelHostPubKey+" allowed-ips 0.0.0.0/0,::/0 endpoint "+t.TunHostMainPublicIP+":"+utils.IToStr(t.WGServerTunnelHostListenPort)+" persistent-keepalive 25", true, true)
								}

								return
							}
						}
					}

					time.Sleep(time.Duration(ping_interval) * time.Second)
				}
			}()
		}
	}

	t.IsInitialised = true

	logger.Info("The tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + " has been setup successfully.")

	return true
}

func (t *Tunnel) Deinit(mode, main_network_interface string, ignoreInitialisationStatus bool) bool {
	if !t.IsInitialised && !ignoreInitialisationStatus {
		logger.Warn("Failed to deinitialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": The tunnel was not initialised. Ignoring tunnel deinitialisation.")

		return false
	}

	if !utils.DoesNetworkInterfaceExist(t.TunnelInterfaceName) {
		logger.Warn("Failed to deinitialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": Tunnel interface does not exist. Ignoring tunnel deinitialisation.")

		return false
	}

	if mode == "tunnel_host" {
		utils.Cmd("iptables-nft -D FORWARD -i "+t.TunnelInterfaceName+" -j ACCEPT", true, true)
		utils.Cmd("iptables-nft -D FORWARD -d "+t.BackendServerTunnelIP+" -m state --state NEW,ESTABLISHED,RELATED -j ACCEPT", true, true)
		utils.Cmd("iptables-nft -D FORWARD -s "+t.BackendServerTunnelIP+" -m state --state NEW,ESTABLISHED,RELATED -j ACCEPT", true, true)

		if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
			utils.Cmd("iptables-nft -t nat -D POSTROUTING -s "+t.TunnelGatewayIP+"/30 ! -o "+t.TunnelInterfaceName+" -j SNAT --to-source "+t.TunHostPublicIP, true, true)
		}

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("iptables-nft -t nat -D POSTROUTING -s "+t.TunnelGatewayIP+"/24 ! -o "+t.TunnelInterfaceName+" -j SNAT --to-source "+t.TunHostPublicIP, true, true)
		}

		if t.TunnelType == "full" {
			utils.Cmd("iptables-nft -t nat -D PREROUTING -d "+t.TunHostPublicIP+" -j DNAT --to-destination "+t.BackendServerTunnelIP, true, true)
		} else {
			for _, port := range t.SplitTunnelPorts {
				p := port["src_port"].(string)
				dp := port["dest_port"].(string)
				proto := port["proto"].(string)

				dstPort := ""

				if len(dp) > 0 {
					dstPort = ":" + dp
				}

				utils.Cmd("iptables-nft -t nat -D PREROUTING -d "+t.TunHostPublicIP+" -p "+proto+" -m "+proto+" --dport "+p+" -j DNAT --to-destination "+t.BackendServerTunnelIP+dstPort, true, true)
			}
		}

		if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
			utils.Cmd("ip addr del "+t.TunHostTunnelIP+"/30 dev "+t.TunnelInterfaceName, true, true)
			utils.Cmd("ip link set "+t.TunnelInterfaceName+" down", true, true)
			utils.Cmd("ip tunnel del "+t.TunnelInterfaceName, true, true)
		}

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("ip addr del "+t.TunHostTunnelIP+"/24 dev "+t.TunnelInterfaceName, true, true)
			utils.Cmd("ip link set "+t.TunnelInterfaceName+" down", true, true)
			utils.Cmd("ip link del "+t.TunnelInterfaceName, true, true)
		}
	}

	if mode == "backend_server" {
		routingTableExists, _ := rttablesCheck(t.TunnelRoutingTablesID, t.TunnelRoutingTablesName)

		if routingTableExists {
			err := rttablesDel(t.TunnelRoutingTablesID, t.TunnelRoutingTablesName)

			if err != nil {
				logger.Warn("Failed to deinitialise the tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + ": Routing table delete failed -> " + err.Error() + ". Continuing tunnel deinitialisation anyway...")
			}
		}

		if t.ShouldRouteAllTrafficThroughTunnel {
			gatewayIP, err := utils.Cmd("ip route show 0.0.0.0/0 dev "+main_network_interface+" | cut -d\\  -f3", true, true)
			if err == nil {
				gatewayIPString := strings.TrimSpace(utils.BytesToString(gatewayIP))

				utils.Cmd("ip route del default via "+t.TunHostTunnelIP+" metric 0", true, true)
				utils.Cmd("ip route del "+t.TunHostMainPublicIP+" via "+gatewayIPString+" dev "+main_network_interface+" onlink", true, true)
			}
		}

		utils.Cmd("ip route del default via "+t.TunHostTunnelIP+" table "+t.TunnelRoutingTablesName, true, true)

		if t.TunnelDriver == "gre" || t.TunnelDriver == "ipip" {
			utils.Cmd("ip rule del from "+t.TunnelGatewayIP+"/30 table "+t.TunnelRoutingTablesName, true, true)
			utils.Cmd("ip addr del "+t.BackendServerTunnelIP+"/30 dev "+t.TunnelInterfaceName, true, true)
			utils.Cmd("ip link set "+t.TunnelInterfaceName+" down", true, true)
			utils.Cmd("ip tunnel del "+t.TunnelInterfaceName, true, true)
		}

		if t.TunnelDriver == "wireguard" {
			utils.Cmd("ip rule del from "+t.TunnelGatewayIP+"/24 table "+t.TunnelRoutingTablesName, true, true)
			utils.Cmd("ip addr del "+t.BackendServerTunnelIP+"/24 dev "+t.TunnelInterfaceName, true, true)
			utils.Cmd("ip link set "+t.TunnelInterfaceName+" down", true, true)
			utils.Cmd("ip link del "+t.TunnelInterfaceName, true, true)
		}
	}

	t.IsInitialised = false

	logger.Info("The tunnel " + t.TunHostMainPublicIP + " <-> " + t.BackendServerPublicIP + " has been removed successfully.")

	return true
}
