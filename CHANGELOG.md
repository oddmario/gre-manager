# Changelog

## v1.20
* Don't enforce txqueuelen/netdev_max_backlog to prevent bufferbloating

## v1.19
* Ensure no race conditions between goroutines and the main function happen

## v1.18.2

## v1.18.1

## v1.18

### Removed
* Don't touch conntrack.

## v1.16

### Changed
* The default qdisc for the tunnel interface(s) to `noqueue` for immediate packet processing.

## v1.15

### Changed
* Updated kernel tuning parameters

## v1.14

## v1.13

### Removed
* Removed the handling of GRO, GSO and TSO. Tunnel Manager no longer touches them.

## v1.12

### Changed
* Versioning format
* Kernel tuning parameters
* Make the— GRO, GSO and TSO disabling —a part of the kernel tunings (won't run unless that option is enabled in config.json)

## v1.1.11

### Added
* Added a logger and omitted the usage of `fmt` for logging.
* Added logging in case of the failure of the execution of any command.

### Changed
* Improved the kernel tuning parameters

## v1.1.10

### Added
* A startup validator that stops Tunnel Manager from running if more than a tunnel have `route_all_traffic_through_tunnel` enabled.

### Fixed
* Fixed the instability that previously occurred when (`route_all_traffic_through_tunnel` + dynamic IPs + multiple tunnels on the same backend) were used.

## v1.1.9

### Changed
* In case of `route_all_traffic_through_tunnel`: Tunnel Manager no longer replaces the default route; it adds a new route with the highest priority instead.

## v1.1.8

### Changed
* Don't set `CGO_ENABLED` to zero while compiling.
* Updated the `build.py` builder to generate more optimised binaries

## v1.1.7

### Added
* Docker image is now built and pushed to [GitHub Container Registry](https://ghcr.io/oddmario/tunnel-manager) ([#1](https://github.com/oddmario/tunnel-manager/pull/1))
* Added Docker Compose example file ([#1](https://github.com/oddmario/tunnel-manager/pull/1))
* Added a workflow for making releases. ([#1](https://github.com/oddmario/tunnel-manager/pull/1))
* Added a workflow for compiling commits and pull requests