package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	initial "myProject/initial"
	crypto "myProject/crypto"
	pt "myProject/protos"

	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 50001, "The server port")
	lst = make(map[int32][]byte, 200) 
)


func init() {
	log.SetFlags(log.LstdFlags |log.Lshortfile |log.LUTC)
}

type MyServer struct {
	pt.UnimplementedBroadAllServer
}

func getF() *os.File {
	filename := fmt.Sprintln("PkList.txt",)
	var f *os.File
	var err error
	if initial.CheckFileIsExist(filename) {
		f, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Printf("write PkAndId error: %v", err)
		}
	} else {
		f, err = os.Create(filename)
		if err != nil {
			panic(err)
		}
	}
	return f
}

func (s *MyServer) BroadPK(re *pt.PkAndId, stream pt.BroadAll_BroadPKServer) error {
	log.Println("pk = ", re.Pk)
	f := getF()
	_, err := fmt.Fprintf(f, "%v, %v\n", re.Id, re.Pk)

	lst[re.Id] = re.Pk
	if err != nil {
		log.Printf("write pkandid in file error: %v", err)
	}
	return nil
}

func (s *MyServer) BroadProof(re *pt.ProofMsg, stream pt.BroadAll_BroadProofServer) error {
	// log.Println("proof = ", re.Proof)
	// log.Println("id = ", re.Id)
	// log.Println("ip = ", re.Ip)

	//verify proof === 如何得到对应的pk
	var data []byte = []byte(re.Proof)
	var proof crypto.VrfProof
	copy(proof[:], data)

	pk_tmp := lst[re.Id]
	var pk crypto.VrfPubkey
	copy(pk[:],pk_tmp)
	ok := initial.VerifyProof(pk, proof, re.Randomness) //证明这个proof确实是pk根据消息randomness产生的
	if !ok {
		log.Fatalf("verify proof error")
	}
	return nil
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pt.RegisterBroadAllServer(grpcServer, &MyServer{})
	grpcServer.Serve(lis)
}
