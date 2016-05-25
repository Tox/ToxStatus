package main

import (
	"flag"
	"os"
)

var (
	networkFlag = flag.String("net", "udp", "network type, either 'udp' or 'tcp'")
	ipFlag      = flag.String("ip", "127.0.0.1", "ip address to probe, ipv4 and ipv6 are both supported")
	portFlag    = flag.Int("port", 33445, "port to probe")
	keyFlag     = flag.String("key", "", "public key of the node")
)

func parseFlags() bool {
	if len(os.Args) < 2 {
		return false
	}

	flag.Parse()

	/*if len(*keyFlag) != 64 {
		log.Fatalln("error: public key must have a lenght of 64 hex characters")
	}

	node := toxNode{}
	node.Ipv4Address = *ipFlag //HACK: ipv6 addresses will also end up here
	node.PublicKey = *keyFlag
	node.Port = *portFlag

	if *networkFlag == "udp" {
		err := probeNode(&node)
		if err == nil {
			log.Println("success: this node appears to be online!")
		} else {
			log.Printf("error: %s", err.Error())
			log.Println("fail: this node appears to be offline!")
		}
	} else if *networkFlag == "tcp" {
		err := probeNodeTCP(&node)
		if err == nil {
			log.Println("success: this relay appears to be online!")
		} else {
			log.Printf("error: %s", err.Error())
			log.Println("fail: this relay appears to be offline!")
		}
	} else {
		log.Fatalf("error: unsupported network specified: %s", *networkFlag)
	}*/

	return true
}
