package internal

import (
	"context"
	"go.uber.org/zap"
	"time"
)

func preCheck(config Config) {

}

func Start(config Config) {
	preCheck(config)
	startMetrics(config)

	s, e := NewRPCServer(context.Background(), config.RPC, "v1.0")
	if e != nil {
		panic(e)
	}
	s.Start()
	loop()
}

func loop() {
	for {
		zap.S().Info("loop .......")
		myT := time.NewTimer(1 * time.Second)
		<-myT.C
		Metrics_errors.WithLabelValues("check_times_count").Add(1)
	}
}
