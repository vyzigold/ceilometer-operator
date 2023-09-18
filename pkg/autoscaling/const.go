package autoscaling

const (
	// ServiceName -
	ServiceName = "aodh"

	// ServiceType -
	ServiceType = "alarming"

	// PrometheusRetention -
	PrometheusRetention = "5h"

	// DatabaseName -
	DatabaseName = "aodh"

	// AodhApiPort -
	AodhApiPort = 8042
)

// PrometheusReplicas -
var PrometheusReplicas int32 = 1
