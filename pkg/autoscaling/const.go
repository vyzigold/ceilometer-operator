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

	// AodhAPIPort -
	AodhAPIPort = 8042
)

// PrometheusReplicas -
var PrometheusReplicas int32 = 1
