package internal

import (
	"go.uber.org/zap"
	"time"
)

func preCheck(config Config) {

}

func Start(config Config) {
	preCheck(config)
	startMetrics(config)
	loop()
}

func loop() {
	for {
		zap.S().Info("loop .......")
		myT := time.NewTimer(1 * time.Second)
		<-myT.C
		errors.WithLabelValues("check_times_count").Add(1)
	}
}
