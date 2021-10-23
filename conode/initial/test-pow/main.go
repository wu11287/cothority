package initial
// package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	crypto "myProject/crypto"
	// "os"
	// "os/exec"
	"sync"
	"time"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
)

const denominator float64 = 18446744073709551615

var lock sync.Mutex

const n int = 400 //表示全网节点数目
const m int = 2 //表示分片的个数


type Node struct {
	Id        int
	weight    int
	Pk        crypto.VrfPubkey
	sk        crypto.VrfPrivkey
	rnd       crypto.VrfOutput
	proof     crypto.VrfProof
	pkList    [n]crypto.VrfPubkey
	proofList [n]crypto.VrfProof
	idList    []int
	shardInNode []int
}

type pkAndId struct {
	id int
	Pk crypto.VrfPubkey
}

type ProofAndId struct {
	id    int
	proof crypto.VrfProof
}

type RndAndId struct {
	id  int
	rnd crypto.VrfOutput
	p   float64
	in  bool
}

//生成长度为 n 的随机byte数组
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func Sotition(nNode *Node, msg []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	lock.Lock()
	proof, ok := nNode.sk.ProveMy(msg)
	if !ok {
		fmt.Println("generate proof error!")
	}

	rnd, ok := proof.Hash()
	if !ok {
		fmt.Println("generate rnd error!")
	}
	nNode.rnd = rnd
	nNode.proof = proof
	lock.Unlock()
}

func newNode(i int) *Node {
	pk_tmp, sk_tmp := crypto.VrfKeygen()

	return &Node {
		Id:			i,
		weight:		rand.Intn(3)+1,
		Pk:			pk_tmp,
		sk: 		sk_tmp,
	}
}

func broadcastPK(ch []chan *pkAndId, nNode *Node, id int, wg *sync.WaitGroup) {
	defer wg.Done()

	lock.Lock()

	tmp := &pkAndId{id, nNode.Pk}
	go func() {
		for i:=0; i< n; i++ {
			ch[i] <- tmp
		}
	}()

	lock.Unlock()
}

func broadcastProof(ch []chan *ProofAndId, nNode *Node, id int, wg *sync.WaitGroup) {
	defer wg.Done()

	lock.Lock()
	tmp := &ProofAndId{id, nNode.proof}

	go func() {
		for i:=0; i < n; i++ {
			ch[i] <- tmp
		}
	}()

	lock.Unlock()
}

func broadcastRnd(ch []chan *RndAndId, nNode *Node, id int, wg *sync.WaitGroup) {
	defer wg.Done()

	lock.Lock()
	var p float64
	for i := 0; i < nNode.weight; i++ {
		p = isMeet(nNode)
		if p > 0.7 {
			continue
		} else {
			break;
		}
	}
	var isin bool

	if p > 0.7 {
		isin = false
	} else {
		isin = true
	}

	tmp := &RndAndId{id, nNode.rnd, p, isin}

	go func() {
		for i:=0; i < n; i++ {
			ch[i] <- tmp
		}
	}()

	lock.Unlock()
}

func isMeet(nNode *Node) float64 {
	bytesBuffer := bytes.NewBuffer(nNode.rnd[:])
	var x int64
	binary.Read(bytesBuffer, binary.BigEndian, &x)

	rnd := float64(x)
	if rnd < 0 {
		rnd += denominator
	}

	p := rnd / denominator
	return p
}

func storePk(ch chan *pkAndId, nNode []*Node, id int, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < n; i++ {
		tmp := <-ch
		nNode[id].pkList[tmp.id] = tmp.Pk
	}
}

func storeProof(ch chan *ProofAndId, nNode []*Node, id int, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < n; i++ {
		tmp := <-ch
		nNode[id].proofList[tmp.id] = tmp.proof
	}
}

func storeRnd(ch chan *RndAndId, nNode []*Node, id int, randomness []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < n; i++ {
		tmp := <-ch
		if tmp.in {
			ok := verifyRnd(nNode[id].pkList[tmp.id], nNode[id].proofList[tmp.id], tmp.rnd, randomness)
			if ok {
				nNode[id].idList = append(nNode[id].idList, tmp.id)
			}
		}
	}
}

func verifyRnd(Pk crypto.VrfPubkey, proof crypto.VrfProof, output crypto.VrfOutput, msg []byte) bool {
	ok, output2 := Pk.VerifyMy(proof, msg)

	if !ok {
		fmt.Println("verified error")
		return false
	}
	return output == output2
}


func Doshard(nNode *Node, idx int, wg *sync.WaitGroup) {
	defer wg.Done()

	mod := idx % m 
	for _, v := range nNode.idList {
		if v % m == mod {
			nNode.shardInNode = append(nNode.shardInNode, v)
		}
	}
}

func count() float64 {
// func main() {
	var wg sync.WaitGroup
	chsPK := make([]chan *pkAndId, n)
	chsProof := make([]chan *ProofAndId, n)
	chsRnd := make([]chan *RndAndId, n)
	nodes := make([]*Node, n)

	//TODO 宿主机调用
	rand.Seed(time.Now().Unix())
	randomness, err := GenerateRandomBytes(10)
	if err != nil {
		fmt.Println("generate random byte[] failed!")
	}

	for i := 0; i < n; i++ {
		chsPK[i] = make(chan *pkAndId)
		chsProof[i] = make(chan *ProofAndId)
		chsRnd[i] = make(chan *RndAndId)
		nodes[i] = newNode(i)
	}

	for i := 0; i < n; i++ {
		wg.Add(2)
		go broadcastPK(chsPK, nodes[i], i, &wg) 
		go storePk(chsPK[i], nodes, i, &wg)
	}

	wg.Wait()

	start := time.Now()
	// 得到cpu使用率
	// command := `../shells/collect_cpu.sh`
	// cmd := exec.Command("/bin/bash", command)
	// err = cmd.Run()
	// if err != nil {
	// 	panic(err)
	// }

	// // 得到系统负载
	// command = `../shells/collect_load.sh`
	// cmd = exec.Command("/bin/bash", command)
	// err = cmd.Run()
	// if err != nil {
	// 	panic(err)
	// }
	go getCpuInfo()
	// go getCpuLoad()

	for i := 0; i < n; i++ {
		wg.Add(1)
		go Sotition(nodes[i], randomness, &wg)
	}

	wg.Wait()


	for i := 0; i < n; i++ {
		wg.Add(2)
		go broadcastProof(chsProof, nodes[i], i, &wg)
		go storeProof(chsProof[i], nodes, i, &wg)
	}
	wg.Wait()


	for i := 0; i < n; i++ {
		wg.Add(2)
		go broadcastRnd(chsRnd, nodes[i], i, &wg)
		go storeRnd(chsRnd[i], nodes, i, randomness, &wg)
	}
	wg.Wait()

	// for _, v := range nodes[0].idList {
	// 	wg.Add(1)
	// 	go Doshard(nodes[v], v, &wg)
	// }
	// wg.Wait()


	// for i := 0; i < m; i++ { //分片个数
	// 	filename := fmt.Sprintf("shard%v.txt", i)
	// 	f, err := os.Create(filename) 
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	defer f.Close()
	// 	for _, v := range nodes[0].idList {
	// 		if v % m == i { //同属于一个分片
	// 			for _, v1 := range nodes[v].shardInNode { 
	// 				_, err = fmt.Fprintf(f, "%v ", v1)
	// 				if err != nil {
	// 					panic(err)
	// 				}
	// 			}
	// 			break
	// 		} else {
	// 			continue
	// 		}
	// 	}
	// }

	interval := time.Since(start) 
	fmt.Printf("time consumed: %v\n", interval)
	time.Sleep(1*time.Second)
	return interval.Seconds()
}


// cpu使用率 + 负载
func getCpuInfo() {
    // CPU使用率
    for i:=0; i < 5; i++ {
        percent, _ := cpu.Percent(time.Second, true)
        fmt.Printf("cpu percent:%v\n", percent)
    }
}

func getCpuLoad() {
    info, _ := load.Avg()
	for {
		fmt.Printf("load: %v\n", info)
	}
}