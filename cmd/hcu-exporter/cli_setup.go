// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"strconv"
	"strings"

	"github.com/HYGON-AI/hcu-exporter/v3/pkg/podresources"
	"github.com/HYGON-AI/hcu-exporter/v3/pkg/util"

	"github.com/golang/glog"
	"github.com/urfave/cli/v2"
)

func beforeCLI(c *cli.Context) error {
	_ = flag.Set("v", strconv.Itoa(c.Int("log-verbose")))
	_ = flag.Set("stderrthreshold", c.String("stderrthreshold"))
	_ = flag.Set("alsologtostderr", c.String("alsologtostderr"))

	var err error
	metricConfig, err = util.NewMetricConfig(c.String("metrics-define"), c.String("label-define"))
	if err != nil {
		return err
	}
	metricsList, err = util.ResolveEnabledMetrics(c.String("metrics-level"), c.String("enable-metrics"))
	if err != nil {
		return err
	}
	glog.Infof("Enabled %d metrics (metrics-level=%q, enable-metrics override=%v)",
		len(metricsList), c.String("metrics-level"), strings.TrimSpace(c.String("enable-metrics")) != "")
	return nil
}

func exporterFlags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:        "pulse",
			Value:       10,
			Usage:       "the metrics collect intervals",
			Destination: &pulse,
			EnvVars:     []string{"PULSE"},
		},
		&cli.IntFlag{
			Name:        "port",
			Value:       16080,
			Destination: &portFlag,
			Usage:       "port number for the exporter",
			EnvVars:     []string{"HCU_EXPORTER_LISTEN"},
		},
		&cli.StringFlag{
			Name: "enable-metrics",
			Usage: "Comma-separated list of internal metric names to enable (e.g. hcu_temp,vhcu_temp). " +
				"When set, overrides --metrics-level. Empty means use --metrics-level preset.",
			Value:   "",
			EnvVars: []string{"ENABLE_METRICS"},
		},
		&cli.StringFlag{
			Name: "metrics-level",
			Usage: "Preset metric set by collection cost: low (fast direct reads), " +
				"medium (low + DF/Hylink/process/health), high (all, including ~1s window metrics). " +
				"Ignored when --enable-metrics is set.",
			Value:   util.MetricsLevelHigh,
			EnvVars: []string{"METRICS_LEVEL"},
		},
		&cli.StringFlag{
			Name: "metrics-define",
			Usage: "JSON map of internal metric names to exported display names, " +
				"e.g. {\"hcu_ce_count\":\"ce_count\",\"hcu_compute_unit_count\":\"compute_unit_count\"}. " +
				"Unspecified metrics keep their original names. Display names must be unique.",
			Value:   "",
			EnvVars: []string{"METRICS_DEFINE"},
		},
		&cli.StringFlag{
			Name: "label-define",
			Usage: "JSON map of global label renames applied to all metrics, " +
				"e.g. {\"block_type\":\"b_type\",\"hcu_pod_name\":\"pod_name\",\"device_id\":\"uuid\"}. " +
				"Display label names must be unique.",
			Value:   "",
			EnvVars: []string{"LABEL_DEFINE"},
		},
		&cli.IntFlag{
			Name:        "sample-duration-ms",
			Usage:       "Sample duration in milliseconds for sampled utilization metrics (default 1000)",
			Value:       1000,
			Destination: &sampleDurationMsFlag,
			EnvVars:     []string{"SAMPLE_DURATION_MS"},
		},
		&cli.StringFlag{
			Name:        "kube-config",
			Usage:       "kube config file",
			Destination: &podresources.Kubeconfig,
			Value:       "/root/.kube/config",
			EnvVars:     []string{"KUBE_CONFIG"},
		},
		&cli.StringFlag{
			Name:    "stderrthreshold",
			Usage:   "log threshold that support:\n\t\t[INFO | WARNING | ERROR]",
			Value:   "INFO",
			EnvVars: []string{"LOG_THRESHOLD"},
		},
		&cli.BoolFlag{
			Name:        "hylink-detail",
			Usage:       "display the detailed information of Hylink or not:\n\t\t[false | true]",
			Value:       false,
			Destination: &hylinkDetailFlag,
			EnvVars:     []string{"HYLINK_DETAIL"},
		},
		&cli.IntFlag{
			Name:    "log-verbose",
			Usage:   "detailed log level support it:\n\t\t(0-10)",
			Value:   2,
			EnvVars: []string{"LOG_VERBOSE"},
		},
		&cli.BoolFlag{
			Name:    "alsologtostderr",
			Usage:   "log outputLog output support it:\n\t\t[false | true]",
			Value:   true,
			EnvVars: []string{"LOG_OUTPUT"},
		},
		&cli.StringFlag{
			Name:        "ips",
			Usage:       "Comma-separated list of allowed IPs or CIDRs for accessing the exporter. If not set, all IPs are allowed.",
			Destination: &allowedIPsFlag,
			EnvVars:     []string{"ALLOWED_IPS"},
		},
		&cli.BoolFlag{
			Name:        "connect-k8s",
			Usage:       "Whether to connect to Kubernetes and collect k8s-related metrics:\n\t\t[false | true]",
			Value:       true,
			Destination: &connectK8sFlag,
			EnvVars:     []string{"CONNECT_K8S"},
		},
	}
}
