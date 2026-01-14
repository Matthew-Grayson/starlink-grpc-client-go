package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/starlink-community/starlink-grpc-go/pkg/spacex.com/api/device"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	addr      string
	timeout   time.Duration
	waitReady bool
}

func parseArgs(args []string) (config, error) {
	fs := flag.NewFlagSet("starlinkctl", flag.ContinueOnError)

	var cfg config
	fs.StringVar(&cfg.addr, "addr", "192.168.100.1:9200", "Starlink dish gRPC address (host:port)")
	fs.DurationVar(&cfg.timeout, "timeout", 3*time.Second, "RPC timeout")
	fs.BoolVar(&cfg.waitReady, "wait-ready", true, "Wait for the channel to become ready (within the timeout) before failing")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	return cfg, nil
}

type dialFn func(addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

func run(out io.Writer, args []string, dial dialFn) error {
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := dial(cfg.addr, opts...)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			_, err := fmt.Fprintf(os.Stderr, "close: %v\n", err)
			if err != nil {
				return
			}
		}
	}()

	client := device.NewDeviceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	req := &device.Request{
		Request: &device.Request_GetStatus{
			GetStatus: &device.GetStatusRequest{},
		},
	}

	var callOpts []grpc.CallOption
	if cfg.waitReady {
		callOpts = append(callOpts, grpc.WaitForReady(true))
	}

	resp, err := client.Handle(ctx, req, callOpts...)
	if err != nil {
		return fmt.Errorf("handle(get_status): %w", err)
	}

	status := resp.GetDishGetStatus()
	if status == nil {
		return fmt.Errorf("unexpected response: dish_get_status is nil")
	}

	info := status.GetDeviceInfo()
	state := status.GetState()

	_, err = fmt.Fprintf(out, "ID: %s\n", info.GetId())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "HW: %s\n", info.GetHardwareVersion())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "SW: %s\n", info.GetSoftwareVersion())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "State: %v\n", state)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Uptime (s): %d\n", status.GetDeviceState().GetUptimeS())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Ping latency (ms): %.2f\n", status.GetPopPingLatencyMs())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Ping drop rate: %.4f\n", status.GetPopPingDropRate())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Downlink (bps): %.0f\n", status.GetDownlinkThroughputBps())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Uplink (bps): %.0f\n", status.GetUplinkThroughputBps())
	if err != nil {
		return err
	}

	obs := status.GetObstructionStats()
	if obs != nil {
		_, err := fmt.Fprintf(out, "Obstructed now: %v\n", obs.GetCurrentlyObstructed())
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "Fraction obstructed: %.4f\n", obs.GetFractionObstructed())
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	err := run(
		io.Writer(os.Stdout),
		os.Args[1:],
		func(addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
			return grpc.NewClient(addr, opts...)
		},
	)
	if err != nil {
		log.Fatalf("%v", err)
	}
}
