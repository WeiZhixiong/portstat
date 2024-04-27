package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var (
	promMetric bool
	interval   int
	// number topN
	number    int
	ipVersion int
)

var rootCmd = cobra.Command{
	Use:   "portstat",
	Short: "monitor tcp available ports",
	Long:  "portstat is a tool to monitor tcp available ports",
	Example: `
  1. portstat
	> Connect                                                                                            UsedPorts  AvailablePorts
	> 192.168.170.132->192.168.170.132:22                                                                2          28229
	> 127.0.0.1->127.0.0.1:22                                                                            1          28230
  2. portstat --prom
    > tcp_used_ports_total{connect="192.168.170.132->192.168.170.132:22"} 2
    > tcp_available_ports_total{connect="192.168.170.132->192.168.170.132:22"} 28229
    > tcp_used_ports_total{connect="127.0.0.1->127.0.0.1:22"} 1
    > tcp_available_ports_total{connect="127.0.0.1->127.0.0.1:22"} 28230
`,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			netVersions []int
		)
		switch ipVersion {
		case 4:
			netVersions = []int{4}
		case 6:
			netVersions = []int{6}
		default:
			netVersions = []int{4, 6}
		}

		if !promMetric {
			fmt.Printf("%-98s %-10s %-5s\n", "Connect", "UsedPorts", "AvailablePorts")
		}

		for {
			topN := number
			portCounters := make([]*PortCounter, 0, number*2)
			for _, netVersion := range netVersions {
				portCountersV, err := GetMiniTcpAvailablePorts(netVersion, topN)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				portCounters = append(portCounters, portCountersV...)
			}

			if len(portCounters) < topN {
				topN = len(portCounters)
			}

			for i := 0; i < topN; i++ {
				minIndex := i
				for j := i + 1; j < len(portCounters); j++ {
					if portCounters[j].availablePorts < portCounters[minIndex].availablePorts {
						minIndex = j
					}
				}
				if minIndex != i {
					portCounters[i], portCounters[minIndex] = portCounters[minIndex], portCounters[i]
				}
			}

			if promMetric {
				OutputPromMetric(topN, portCounters)
				break
			}

			for i := 0; i < topN; i++ {
				fmt.Printf("%-98s %-10d %-5d\n", portCounters[i].connectID, portCounters[i].usedPorts, portCounters[i].availablePorts)
			}

			time.Sleep(time.Duration(interval) * time.Second)
		}
	},
}

func init() {
	rootCmd.Flags().IntVarP(&interval, "interval", "i", 3, `monitor interval, unit is seconds`)
	rootCmd.Flags().IntVarP(&number, "number", "n", 10, `outputs the topN smallest available ports`)
	rootCmd.Flags().BoolVarP(&promMetric, "prom", "p", false, "output in prometheus metrics format. only once, interval doesn't work")
	rootCmd.Flags().IntVarP(&ipVersion, "ipVersion", "e", 0, `ip version, 4 or 6, default is 0, means both`)
}

func main() {
	rootCmd.Execute()
}
