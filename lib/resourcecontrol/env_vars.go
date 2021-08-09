package resourcecontrol

import "os"

type envVar struct {
	name  string
	value string
}

func (ev *envVar) String() string {
	return ev.name + "=" + ev.value
}

func genLimitsEnvVars(limits ResourceLimits) []envVar {
	return []envVar{
		{cpuMaxEnvVar, limits.CPUMax},
		{memMaxEnvVar, limits.MemMax},
		{ioMaxRbpsEnvVar, limits.IOMaxRbps},
		{ioMaxWbpsEnvVar, limits.IOMaxWbps},
	}
}

func unsetCustomEnvVars() {
	for _, v := range []string{execEnvVar, cpuMaxEnvVar, memMaxEnvVar, ioMaxRbpsEnvVar, ioMaxWbpsEnvVar} {
		os.Unsetenv(v)
	}
}
