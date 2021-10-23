package initial

import (
	"bytes"
	"io"
	"os"
	"context"
	"encoding/binary"
	"log"
	"math/rand"
	"sync"
	"time"

	crypto "cothority/conode/crypto"
	pt "cothority/conode/protos"
)

const denominator float64 = 18446744073709551615

var lock sync.Mutex

type Node struct {
	Id        int
	Weight    int
	Pk        crypto.VrfPubkey
	Sk        crypto.VrfPrivkey
	// PkList    []crypto.VrfPubkey
	Rnd       crypto.VrfOutput
	Proof     crypto.VrfProof
	Choosed   bool
	IpInShard []IPAndId
}

type IPAndId struct {
	Ip string
	Id int
}

type PkAndId struct {
	Id int
	Pk crypto.VrfPubkey
}

type ProofAndId struct {
	Id    int
	Proof crypto.VrfProof
}

func NewNode(i int) *Node {
	pk, sk := crypto.VrfKeygen()
	return &Node{
		Id:     i,
		Weight: rand.Intn(3) + 1,
		Pk:     pk,
		Sk:     sk,
	}
}

func CheckFileIsExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

func RunBroadPk(client pt.BroadAllClient, node *Node) {
	tmp := string(node.Pk[:])
	var data []byte = []byte(tmp)

	
	msg := &pt.PkAndId {
		Pk: data,
		Id: int32(node.Id),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.BroadPK(ctx, msg)
	if err != nil {
		log.Fatalf("%v.BroadPK(_) = _, %v", client, err)
	}

	for {
		feature, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.RunBroadPk(_) = _, %v", client, err)
		}
		log.Println(feature)
	}
}


func RunBroadProof(client pt.BroadAllClient, node *Node, randomness []byte, ip string, id int) {
	proof_tmp := string(node.Proof[:])
	var data []byte = []byte(proof_tmp)

	var ip_end []byte = []byte(ip)
	msg := &pt.ProofMsg{
		Proof: data, 
		Id: int32(node.Id), 
		Ip: ip_end,
		Randomness: randomness,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.BroadProof(ctx, msg)
	if err != nil {
		log.Fatalf("%v.BroadProof(_) = _, %v", client, err)
	}

	for {
		feature, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.RunBroadPk(_) = _, %v", client, err)
		}
		log.Println(feature)
	}
}

// 传播的时候没有传播rnd
func VerifyProof(Pk crypto.VrfPubkey, proof crypto.VrfProof, msg []byte) bool {
	ok, _:= Pk.VerifyMy(proof, msg)

	if !ok {
		log.Println("verified error")
		return false
	}
	return ok
}

// msg 是全局一致的随机值
func Sotition(node *Node, msg []byte, wg *sync.WaitGroup) {
	// defer wg.Done()
	lock.Lock()

	proof, ok := node.Sk.ProveMy(msg)
	if !ok {
		log.Fatal("generate proof error")
	}

	rnd, ok := proof.Hash()
	if !ok {
		log.Fatal("generate rnd error")
	}
	node.Rnd = rnd
	node.Proof = proof

	log.Println("node.weight = ", node.Weight)
	for i := 0; i < node.Weight; i++ {
		ok = VerifyRnd(node)
		if ok {
			node.Choosed = true
			break
		}
	}

	lock.Unlock()
}

// 判断该节点是否被选中
func VerifyRnd(node *Node) bool {
	bytesBuffer := bytes.NewBuffer(node.Rnd[:])
	var x int64
	binary.Read(bytesBuffer, binary.BigEndian, &x)

	rnd := float64(x)
	if rnd < 0 {
		rnd += denominator
	}

	p := rnd / denominator //得到一个概率值
	log.Println("sortition success, I am in system")

	return p < 0.7
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}
