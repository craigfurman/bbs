package testrunner

import (
	"os/exec"
	"time"

	"github.com/tedsuo/ifrit/ginkgomon"
)

type Args struct {
	Address               string
	AdvertiseURL          string
	AuctioneerAddress     string
	ConsulCluster         string
	DropsondeDestination  string
	EtcdCACert            string
	EtcdClientCert        string
	EtcdClientKey         string
	EtcdCluster           string
	MetricsReportInterval time.Duration
}

func (args Args) ArgSlice() []string {
	return []string{
		"-advertiseURL", args.AdvertiseURL,
		"-auctioneerAddress", args.AuctioneerAddress,
		"-consulCluster", args.ConsulCluster,
		"-dropsondeDestination", args.DropsondeDestination,
		"-etcdCaFile", args.EtcdCACert,
		"-etcdCertFile", args.EtcdClientCert,
		"-etcdCluster", args.EtcdCluster,
		"-etcdKeyFile", args.EtcdClientKey,
		"-listenAddress", args.Address,
		"-logLevel", "debug",
		"-metricsReportInterval", args.MetricsReportInterval.String(),
	}
}

func New(binPath string, args Args) *ginkgomon.Runner {
	if args.MetricsReportInterval == 0 {
		args.MetricsReportInterval = time.Minute
	}
	return ginkgomon.New(ginkgomon.Config{
		Name:       "bbs",
		Command:    exec.Command(binPath, args.ArgSlice()...),
		StartCheck: "bbs.started",
	})
}
