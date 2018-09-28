/*
Package reporters : report state of kubernetes using MQTT.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package reporters

/*
ReporterInf : a interface to specify the method signatures that reporter should be implemented.
*/
type ReporterInf interface {
	GetAttrsTopic() string
	StartReporting()
	GetStopCh() chan bool
	GetFinishCh() chan bool
}

type baseReporter struct {
	deviceType string
	deviceID   string
}

/*
GetAttrsTopic : get the attributes topic name
*/
func (b *baseReporter) GetAttrsTopic() string {
	return "/" + b.deviceType + "/" + b.deviceID + "/attrs"
}
