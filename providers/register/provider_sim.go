// +build !no_sim_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/sim"
)

func init() {
	register("sim", initSim)
}

func initSim(cfg InitConfig) (providers.Provider, error) {
	return sim.NewSimProvider(
		cfg.ConfigPath,
		cfg.NodeName,
		cfg.InternalIP,
		cfg.DaemonPort,
	)
}
