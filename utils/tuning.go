package utils

func SysTuning(shouldEnableIPIPmod, shouldEnableGREmod, shouldEnableWGmod bool, mode, mainNetworkInterface string, applyTuningTweaks bool) {
	if shouldEnableGREmod {
		Cmd("modprobe ip_gre", true, true)
	}

	if shouldEnableIPIPmod {
		Cmd("modprobe ipip", true, true)
	}

	if shouldEnableWGmod {
		Cmd("modprobe wireguard", true, true)
	}

	if mode == "tunnel_host" {
		Cmd("sysctl -w net.ipv4.ip_forward=1", true, true)
		Cmd("sysctl -w net.ipv4.conf."+mainNetworkInterface+".proxy_arp=1", true, true)
		Cmd("sysctl -w net.ipv4.conf.all.rp_filter=0", true, true)
		Cmd("sysctl -w net.ipv4.conf.default.rp_filter=0", true, true)
		Cmd("sysctl -w net.ipv4.conf.all.accept_redirects=0", true, true)
		Cmd("sysctl -w net.ipv4.conf.default.accept_redirects=0", true, true)

		if applyTuningTweaks {
			Cmd("modprobe ip_conntrack", true, true)
			Cmd("modprobe nf_conntrack", true, true)

			Cmd("sysctl -w fs.file-max=2097152", true, true)
			Cmd("sysctl -w fs.inotify.max_user_instances=2097152", true, true)
			Cmd("sysctl -w fs.inotify.max_user_watches=2097152", true, true)
			Cmd("sysctl -w fs.nr_open=2097152", true, true)
			Cmd("sysctl -w fs.aio-max-nr=2097152", true, true)
			Cmd("sysctl -w net.core.somaxconn=65535", true, true)
			Cmd("sysctl -w net.ipv4.ip_local_port_range=\"16384 65535\"", true, true)
			Cmd("sysctl -w net.nf_conntrack_max=1000000", true, true)
			Cmd("sysctl -w net.netfilter.nf_conntrack_max=1000000", true, true)
			Cmd("sysctl -w net.nf_conntrack_buckets=1000000", true, true)
			Cmd("sysctl -w net.netfilter.nf_conntrack_buckets=1000000", true, true)

			Cmd("sysctl -w net.ipv4.route.flush=1", true, true)
			Cmd("sysctl -w net.ipv6.route.flush=1", true, true)
		}

		Cmd("iptables-nft -F", true, true)
	}
}
