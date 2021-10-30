package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	initial "myProject/initial"
	pt "myProject/protos"

	"google.golang.org/grpc"
)

const id int = 2

var (
	serverAddr = flag.String("server_addr", "172.24.0.3:50001", "The server address in the format of host:port")
)


func main() {
	var wg sync.WaitGroup
	
	node := initial.NewNode(id)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}
	ip := addrs[1]
	log.Println(ip) 

// TODO 如何达到对所有ip的轮询
	// for _,addr:=range addrs{
	// 	if ipnet,ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback(){
	// 		if ipnet.IP.To4() != nil {
	// 			fmt.Println(ipnet.IP.String())
	// 		}
	// 	}
	// }

	conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("create conn error : %v", err)
	}
	defer conn.Close()
	client := pt.NewBroadAllClient(conn)
	initial.RunBroadPk(client, node)

	randomness, err := initial.GenerateRandomBytes(10) //不用seed会产生确定性结果,作为初始状态下传递的消息
	if err != nil {
		log.Fatalf("generate randomness error: %v", err)
	}

	start := time.Now()
	// sortition -- 加密抽签成功的节点才会调用广播proof的方法
	initial.Sotition(node, randomness, &wg)
	if node.Choosed {
		initial.RunBroadProof(client, node, randomness, ip.String(), id)
	}
	interval := time.Since(start) 
	fmt.Printf("time consumed: %v\n", interval)
	wg.Wait()
}