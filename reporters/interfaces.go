/*
Package reporters : report state of kubernetes using MQTT.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package reporters

import (
	"time"
)

/*
ReporterInf : a interface to specify the method signatures that Reporter should be implemented.
*/
type ReporterInf interface {
	GetAttrsTopic() string
	StartReporting()
	GetStopCh() chan bool
	GetFinishCh() chan bool
}

/*
ReporterImplInf : a interface to specify the method signatures that ReporterImpl should be implemented.
*/
type ReporterImplInf interface {
	Report(string)
}

type baseReporter struct {
	deviceType       string
	deviceID         string
	intervalMillisec time.Duration
	stopCh           chan bool
	finishCh         chan bool
}

/*
GetAttrsTopic : get the attributes topic name
*/
func (b *baseReporter) GetAttrsTopic() string {
	return "/" + b.deviceType + "/" + b.deviceID + "/attrs"
}

/*
GetStopCh : get the channel to receive a loop stop message
*/
func (b *baseReporter) GetStopCh() chan bool {
	return b.stopCh
}

/*
GetFinishCh : get the channel to send a loop stopped message
*/
func (b *baseReporter) GetFinishCh() chan bool {
	return b.finishCh
}

func (b *baseReporter) loop(impl ReporterImplInf) {
	ticker := time.NewTicker(b.intervalMillisec * time.Millisecond)

LOOP:
	for {
		select {
		case <-ticker.C:
			impl.Report(b.GetAttrsTopic())
		case <-b.stopCh:
			ticker.Stop()
			break LOOP
		}
	}

	close(b.stopCh)
	close(b.finishCh)
}
